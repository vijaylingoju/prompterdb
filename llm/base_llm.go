package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/vijaylingoju/prompterdb/templates"
)

type BaseLLM struct {
	name            string
	templateManager *templates.TemplateManager
}

func NewBaseLLM(name string) *BaseLLM {
	return &BaseLLM{
		name:            name,
		templateManager: templates.NewTemplateManager(),
	}
}

func (b *BaseLLM) Name() string {
	return b.name
}

func (b *BaseLLM) SetTemplateManager(tm *templates.TemplateManager) {
	b.templateManager = tm
}

// preparePrompt prepares the prompt using the template system
func (b *BaseLLM) preparePrompt(req QueryRequest) (string, error) {
	if b.templateManager == nil {
		return "", fmt.Errorf("template manager not set")
	}

	templateName := req.Template
	if templateName == "" {
		templateName = "default"
	}
	var templateType templates.TemplateType
	if req.QueryType == QueryTypeMongo {
		templateType = templates.MongoSystemPrompt
	} else {
		templateType = templates.SystemPrompt
	}

	// Prepare template data
	data := map[string]interface{}{
		"Schema":      req.Schema,
		"UserRequest": req.Prompt,
		"DBType":      req.DBType,
	}

	// Add custom variables to template data
	for k, v := range req.CustomVars {
		data[k] = v
	}

	// Execute the template
	prompt, err := b.templateManager.ExecuteTemplate(
		templateType,
		req.DBType,
		templateName,
		data,
	)
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return prompt, nil
}

// validateAndDebugResponse validates the response and logs debug information if enabled
func (b *BaseLLM) validateAndDebugResponse(response string) bool {
	isValid := json.Valid([]byte(response))
	if !isValid {
		log.Println("=== Response Validation Failed ===")
		log.Printf("Response is not valid JSON")
		log.Printf("Raw response: %s", response)
		log.Println("================================")
		return false
	}

	// Only log full response in debug mode
	if os.Getenv("DEBUG_LLM") == "true" {
		var prettyJSON bytes.Buffer
		err := json.Indent(&prettyJSON, []byte(response), "", "  ")
		if err == nil {
			log.Println("=== LLM Response ===")
			log.Printf("\n%s\n", prettyJSON.String())
			log.Println("====================")
		}
	}
	return true
}

// formatResponse formats the response using the template system
func (b *BaseLLM) formatResponse(req QueryRequest, query, explanation string) (string, error) {
	if b.templateManager == nil {
		return "", fmt.Errorf("template manager not set")
	}

	templateName := req.Template
	if templateName == "" {
		templateName = "default"
	}

	templateType := templates.ResponseFormat
	if req.QueryType == QueryTypeMongo {
		templateType = templates.MongoResponseFormat
	}

	// For MongoDB, parse the query to extract collection and operation if possible
	if req.QueryType == QueryTypeMongo {
		// Try to parse the query as JSON to extract collection and operation
		var mongoQuery map[string]interface{}
		if err := json.Unmarshal([]byte(query), &mongoQuery); err == nil {
			// If the query is valid JSON, use it directly
			mongoQuery["explanation"] = explanation
			mongoQuery["timestamp"] = time.Now().Format(time.RFC3339)
			mongoQuery["database"] = "mongodb"
			
			// Ensure required fields are present
			if _, ok := mongoQuery["collection"]; !ok {
				mongoQuery["collection"] = ""
			}
			if _, ok := mongoQuery["operation"]; !ok {
				mongoQuery["operation"] = "find"
			}
			if _, ok := mongoQuery["filter"]; !ok {
				mongoQuery["filter"] = map[string]interface{}{}
			}
			
			// Convert back to JSON
			jsonData, err := json.MarshalIndent(mongoQuery, "", "  ")
			if err == nil {
				return string(jsonData), nil
			}
		}
	}

	// If no template is found or this is not a MongoDB query, return a default formatted response
	if !b.templateManager.HasTemplate(templateType, strings.ToLower(req.DBType), templateName) {
		result := map[string]interface{}{
			"query":       query,
			"explanation": explanation,
			"timestamp":   time.Now().Format(time.RFC3339),
			"database":    req.DBType,
		}
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		return string(jsonData), nil
	}

	// Prepare template data with all required fields
	data := map[string]interface{}{
		"Query":       query,
		"Explanation": explanation,
		"DBType":      req.DBType,
		"Timestamp":   time.Now().Format(time.RFC3339),
		"Database":    req.DBType, // This will be used for the "database" field in the template
	}

	// Add type-specific fields
	switch req.QueryType {
	case QueryTypeSQL:
		// Initialize empty parameters array that can be filled by query parsing if needed
		data["Parameters"] = []interface{}{}
	case QueryTypeMongo:
		// Default values for MongoDB
		data["Collection"] = ""
		data["Operation"] = "find"
		data["Filter"] = map[string]interface{}{}
	}

	// Add custom variables to template data
	for k, v := range req.CustomVars {
		data[k] = v
	}

	// Handle parameters - extract them from the query if they exist
	parameters, remainingQuery := extractParameters(query)
	if len(parameters) > 0 {
		query = remainingQuery
		data["Parameters"] = parameters
	}

	// Convert parameters to JSON for the template
	if params, ok := data["Parameters"]; ok {
		if jsonData, err := json.Marshal(params); err == nil {
			data["ParametersJSON"] = string(jsonData)
		}
	} else {
		data["ParametersJSON"] = "[]"
	}

	// Ensure explanation is never empty
	if explanation == "" {
		explanation = "No explanation provided"
	}
	data["Explanation"] = explanation

	// Execute the template
	tmplResult, err := b.templateManager.ExecuteTemplate(
		templateType,
		req.DBType,
		templateName,
		data,
	)
	if err != nil {
		return "", fmt.Errorf("error formatting response: %w", err)
	}

	// Validate and debug the response if enabled
	if os.Getenv("DEBUG_LLM") != "" {
		b.validateAndDebugResponse(tmplResult)
	}

	return tmplResult, nil
}

// extractParameters extracts parameters from a query string.
// Returns the parameters and the remaining query string.
func extractParameters(query string) ([]interface{}, string) {
	re := regexp.MustCompile(`\$\d+`)
	matches := re.FindAllString(query, -1)
	if len(matches) == 0 {
		return nil, query
	}

	// Remove parameters from the query
	cleanQuery := re.ReplaceAllString(query, "?")
	
	// Extract parameter values (for now, using placeholders)
	// In a real implementation, you'd extract these from the query
	params := make([]interface{}, 0, len(matches))
	for i := range matches {
		params = append(params, fmt.Sprintf("param%d", i+1))
	}

	return params, cleanQuery
}

// getStringValue safely extracts a string value from a map with a default fallback
func getStringValue(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}
