// llm/openai.go
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/vijaylingoju/prompterdb/templates"
)

type OpenAI struct {
	*BaseLLM
	Client *openai.Client
	Model  string
}

func NewOpenAI(apiKey, model string) (*OpenAI, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.HTTPClient = &http.Client{}
	
	return &OpenAI{
		BaseLLM: NewBaseLLM("openai"),
		Client:  openai.NewClientWithConfig(cfg),
		Model:   model,
	}, nil
}

// Name returns the name of the LLM implementation
func (o *OpenAI) Name() string {
	return "openai"
}

// SetTemplateManager sets the template manager for the LLM
func (o *OpenAI) SetTemplateManager(tm *templates.TemplateManager) {
	o.BaseLLM.templateManager = tm
}

func (o *OpenAI) GenerateQuery(req QueryRequest) (*QueryResponse, error) {
	if o.templateManager == nil {
		return nil, errors.New("template manager not set")
	}

	// Prepare the prompt using the template system
	prompt, err := o.preparePrompt(req)
	if err != nil {
		return nil, fmt.Errorf("error preparing prompt: %w", err)
	}

	// Call the OpenAI API
	resp, err := o.Client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: o.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a database query generator. Generate only the requested query without any additional text or explanation.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.1, // Lower temperature for more deterministic output
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response from OpenAI")
	}

	query := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Format the response using the template system
	formattedResponse, err := o.formatResponse(req, query, "")
	if err != nil {
		return nil, fmt.Errorf("error formatting response: %w", err)
	}

	// Parse the formatted response if it's JSON
	var responseMap map[string]interface{}
	if err := json.Unmarshal([]byte(formattedResponse), &responseMap); err != nil {
		// If it's not valid JSON, just use it as is
		responseMap = map[string]interface{}{
			"query":       query,
			"explanation": "",
			"rawResponse": formattedResponse,
		}
	}

	// Create the response object
	response := &QueryResponse{
		Query:       getStringValue(responseMap, "query", query),
		Explanation: getStringValue(responseMap, "explanation", ""),
		RawResponse: formattedResponse,
	}

	return response, nil
}


