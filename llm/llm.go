// llm/llm.go
package llm

import (
	"github.com/vijaylingoju/prompterdb/templates"
)

type QueryType string

const (
	QueryTypeSQL    QueryType = "sql"
	QueryTypeMongo  QueryType = "mongo"
)

type QueryRequest struct {
	Prompt      string
	Schema      string
	DBType      string
	QueryType   QueryType
	Template    string // Optional: name of the template to use
	CustomVars  map[string]interface{} // Additional variables for template
}

type QueryResponse struct {
	Query       string
	Explanation string
	RawResponse string
}

type LLM interface {
	// GenerateQuery generates a database query based on the request
	GenerateQuery(req QueryRequest) (*QueryResponse, error)
	
	// Name returns the name/identifier of the LLM implementation
	Name() string
	
	// SetTemplateManager sets the template manager to use
	SetTemplateManager(tm *templates.TemplateManager)
}
