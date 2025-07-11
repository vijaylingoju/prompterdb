// llm/openai.go
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAI struct {
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
		Client: openai.NewClientWithConfig(cfg),
		Model:  model,
	}, nil
}

func (o *OpenAI) Name() string {
	return "openai"
}

func (o *OpenAI) GenerateSQL(prompt string, schema string) (string, error) {
	fullPrompt := fmt.Sprintf("Given the following DB schema:\n%s\nGenerate a SELECT SQL query for: %s", schema, prompt)

	resp, err := o.Client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: o.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "You are a SQL generator. Only respond with SQL SELECT queries."},
			{Role: "user", Content: fullPrompt},
		},
	})
	if err != nil {
		return "", err
	}

	query := strings.TrimSpace(resp.Choices[0].Message.Content)
	return query, nil
}
