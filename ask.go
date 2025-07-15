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
)

func Ask(userPrompt string, llmClient llm.LLM) ([]map[string]interface{}, error) {
	if userPrompt == "" {
		return nil, errors.New("prompt is empty")
	}

	schema := GetAllSchemas()
	if schema == "" {
		return nil, errors.New("schema is empty – call IntrospectAllSchemas() first")
	}

	// STEP 1: Route to the most appropriate DB
	targetDB, err := engine.RoutePrompt(context.Background(), userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to determine target database: %w", err)
	}

	if targetDB.Name == "" || targetDB.Type == "" {
		return nil, errors.New("invalid database configuration")
	}

	switch targetDB.Type {
	case config.Postgres:
		// Step 2: Ask LLM to generate SQL
		query, err := llmClient.GenerateSQL("Only return a valid SQL query without any explanation. "+userPrompt, schema)
		if err != nil {
			return nil, fmt.Errorf("llm generation failed: %w", err)
		}

		query = cleanLLMQuery(query)

		// Step 3: Validate SQL
		if err := llm.ValidateSQL(query); err != nil {
			return nil, fmt.Errorf("query validation failed: %w", err)
		}

		// Step 4: Execute SQL
		isSelect := strings.HasPrefix(strings.ToLower(strings.TrimSpace(query)), "select")
		if isSelect {
			return db.QueryPostgres(targetDB.Name, query)
		}
		rowsAffected, err := db.Execute(targetDB.Name, query)
		if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
		return []map[string]interface{}{{"status": "success", "rows_affected": rowsAffected}}, nil

	case config.Mongo:
		// Prompt to instruct LLM to give JSON with operation
		llmTemplate := `
You are an AI assistant that converts natural language into MongoDB operations.
⚠️ Only respond with a valid JSON. Do NOT add explanation or markdown.

JSON format must include:
- "operation": one of ["find", "insert", "update", "delete"]
- "collection": the collection name
- For "find": "filter"
- For "insert": "document"
- For "update": "filter" and "update"
- For "delete": "filter"

Examples:

Prompt: find all courses  
Response: { "operation": "find", "collection": "courses", "filter": {} }

Prompt: add new course with title mongodb and duration 4  
Response: { "operation": "insert", "collection": "courses", "document": { "title": "mongodb", "duration": 4 } }

Prompt: update duration of mongodb course to 6  
Response: { "operation": "update", "collection": "courses", "filter": { "title": "mongodb" }, "update": { "$set": { "duration": 6 } } }

Prompt: delete course titled mongodb  
Response: { "operation": "delete", "collection": "courses", "filter": { "title": "mongodb" } }

Prompt: {{userPrompt}}
`

		finalPrompt := strings.ReplaceAll(llmTemplate, "{{userPrompt}}", userPrompt)

		rawResponse, err := llmClient.GenerateMongoQuery(finalPrompt, schema)
		if err != nil {
			return nil, fmt.Errorf("mongo query generation failed: %w", err)
		}

		// Clean and validate the MongoDB query
		cleanedQuery := cleanMongoText(rawResponse)
		if err := llm.ValidateMongo(cleanedQuery); err != nil {
			return nil, fmt.Errorf("mongo query validation failed: %w", err)
		}

		log.Println(" Raw Mongo LLM response:", rawResponse)
		cleaned := cleanMongoText(rawResponse)

		var mongoQuery struct {
			Operation  string                 `json:"operation"`
			Collection string                 `json:"collection"`
			Filter     map[string]interface{} `json:"filter,omitempty"`
			Document   map[string]interface{} `json:"document,omitempty"`
			Update     map[string]interface{} `json:"update,omitempty"`
		}

		if err := json.Unmarshal([]byte(cleaned), &mongoQuery); err != nil {
			return nil, fmt.Errorf("error parsing LLM Mongo response: %w\nRaw cleaned: %s", err, cleaned)
		}

		if mongoQuery.Collection == "" {
			mongoQuery.Collection = FindMostRelevantMongoCollection(userPrompt, targetDB.Name)
			if mongoQuery.Collection == "" {
				return nil, errors.New("could not infer MongoDB collection name from prompt")
			}
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
