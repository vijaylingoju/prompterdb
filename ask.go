package prompterdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/vijaylingoju/prompterdb/config"
	"github.com/vijaylingoju/prompterdb/db"
	"github.com/vijaylingoju/prompterdb/engine"
	"github.com/vijaylingoju/prompterdb/llm"
	"github.com/vijaylingoju/prompterdb/templates"
	"go.mongodb.org/mongo-driver/bson"
)

// Ask processes a natural language query and returns the results along with visualization suggestions
func Ask(userPrompt string, llmClient llm.LLM) ([]map[string]interface{}, error) {
	if userPrompt == "" {
		return nil, errors.New("prompt is empty")
	}

	schema := GetAllSchemas()
	if schema == "" {
		return nil, errors.New("schema is empty – call IntrospectAllSchemas() first")
	}

	// Initialize template manager
	tm := templates.NewTemplateManager()
	// Load templates from the default directory
	if err := tm.LoadTemplatesFromDir("templates"); err != nil {
		log.Printf("Warning: could not load templates: %v", err)
	}

	// Set the template manager for the LLM client
	llmClient.SetTemplateManager(tm)

	// STEP 1: Route to the most appropriate DB
	targetDB, err := engine.RoutePrompt(context.Background(), userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to determine target database: %w", err)
	}

	if targetDB.Name == "" || targetDB.Type == "" {
		return nil, errors.New("invalid database configuration")
	}

	// Prepare the query request
	req := llm.QueryRequest{
		Prompt:     userPrompt,
		Schema:     schema,
		DBType:     strings.ToLower(string(targetDB.Type)),
		CustomVars: make(map[string]interface{}),
	}

	switch targetDB.Type {
	case config.Postgres:
		req.QueryType = llm.QueryTypeSQL
		// Step 2: Ask LLM to generate SQL
		resp, err := llmClient.GenerateQuery(req)
		if err != nil {
			return nil, fmt.Errorf("llm generation failed: %w", err)
		}

		query := cleanLLMQuery(resp.Query)

		// Step 3: Validate SQL
		if err := llm.ValidateSQL(query); err != nil {
			return nil, fmt.Errorf("query validation failed: %w", err)
		}

		// Step 4: Execute SQL
		isSelect := strings.HasPrefix(strings.ToLower(strings.TrimSpace(query)), "select")
		if isSelect {
			return db.QueryPostgres(targetDB.Name, query)
		}
		// For non-SELECT queries, execute and return the result
		rowsAffected, err := db.Execute(targetDB.Name, query)
		if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
		results := []map[string]interface{}{
			{"rows_affected": rowsAffected},
		}
		if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}

		// If there are results, try to generate visualizations
		if len(results) > 0 {
			log.Println("Generating visualization suggestions...")
			// Initialize template manager
			tm := templates.NewTemplateManager()
			if err := tm.LoadTemplatesFromDir("templates"); err != nil {
				log.Printf("Warning: could not load templates for visualization: %v", err)
			}

			// Generate visualization suggestions
			widgets, vizErr := Visualize(results, "default", tm, llmClient)
			if vizErr != nil {
				log.Printf("Warning: could not generate visualizations: %v", vizErr)
			} else if len(widgets) > 0 {
				log.Println("\n=== Visualization Suggestions ===")
				PrintWidgetConfig(widgets)
			}
		}

		return results, nil

	case config.Mongo:
		req.QueryType = llm.QueryTypeMongo

		// Find the most relevant collection if not specified
		collection := FindMostRelevantMongoCollection(userPrompt, targetDB.Name)
		if collection == "" {
			return nil, errors.New("could not infer MongoDB collection name from prompt")
		}
		req.CustomVars["Collection"] = collection

		// Step 2: Generate MongoDB query using the template system
		resp, err := llmClient.GenerateQuery(req)
		if err != nil {
			return nil, fmt.Errorf("mongo query generation failed: %w", err)
		}

		// Clean and validate the MongoDB query
		cleanedQuery := cleanMongoText(resp.Query)
		if err := llm.ValidateMongo(cleanedQuery); err != nil {
			return nil, fmt.Errorf("mongo query validation failed: %w", err)
		}

		log.Println("Raw Mongo LLM response:", cleanedQuery)

		var mongoQuery struct {
			Operation  string                 `json:"operation"`
			Collection string                 `json:"collection"`
			Filter     map[string]interface{} `json:"filter,omitempty"`
			Document   map[string]interface{} `json:"document,omitempty"`
			Update     map[string]interface{} `json:"update,omitempty"`
			Pipeline   []bson.M               `json:"pipeline,omitempty"`
		}

		if err := json.Unmarshal([]byte(cleanedQuery), &mongoQuery); err != nil {
			return nil, fmt.Errorf("error parsing LLM Mongo response: %w\nRaw response: %s", err, cleanedQuery)
		}

		// Use the collection from the template if not specified in the response
		if mongoQuery.Collection == "" {
			mongoQuery.Collection = collection
		}

		switch mongoQuery.Operation {
		case "find":
			return db.QueryMongo(targetDB.Name, targetDB.DBName, mongoQuery.Collection, mongoQuery.Filter)
		case "insert":
			return db.InsertMongo(targetDB.Name, targetDB.DBName, mongoQuery.Collection, mongoQuery.Document)
		case "update":
			return db.UpdateMongo(targetDB.Name, targetDB.DBName, mongoQuery.Collection, mongoQuery.Filter, mongoQuery.Update)
		case "delete":
			return db.DeleteMongo(targetDB.Name, targetDB.DBName, mongoQuery.Collection, mongoQuery.Filter)
		case "aggregate":
			return db.AggregateMongo(targetDB.Name, targetDB.DBName, mongoQuery.Collection, mongoQuery.Pipeline)
		default:
			return nil, fmt.Errorf("unsupported Mongo operation: %s", mongoQuery.Operation)
		}

	default:
		return nil, fmt.Errorf("unsupported DB type: %s", targetDB.Type)
	}
}

func cleanLLMQuery(raw string) string {
	lines := strings.Split(raw, "\n")
	cleaned := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasSuffix(line, "```") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func cleanMongoText(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	idx := strings.Index(response, "{")
	if idx > 0 {
		response = response[idx:]
	}

	lines := strings.Split(response, "\n")
	var clean []string
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "note") || strings.HasPrefix(strings.ToLower(line), "however") {
			break
		}
		clean = append(clean, line)
	}
	return strings.Join(clean, "\n")
}

func FindMostRelevantMongoCollection(prompt string, dbName string) string {
	prompt = strings.ToLower(prompt)

	cfg, ok := config.RegisteredDBs[dbName]
	if !ok || cfg.Type != config.Mongo {
		return ""
	}

	schema, err := db.GetMongoSchema(dbName)
	if err != nil || schema == "" {
		return ""
	}

	bestMatch := ""
	highestScore := 0

	lines := strings.Split(schema, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		lineLower := strings.ToLower(line)
		score := 0
		for _, word := range strings.Fields(prompt) {
			if strings.Contains(lineLower, word) {
				score++
			}
		}
		if score > highestScore {
			highestScore = score
			bestMatch = strings.SplitN(line, "(", 2)[0]
		}
	}
	return bestMatch
}
