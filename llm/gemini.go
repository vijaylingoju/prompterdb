// llm/gemini.go
package llm

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Gemini struct {
	Client *genai.Client
	Model  *genai.GenerativeModel
}

func NewGemini(apiKey string) (*Gemini, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("missing Google API key")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	return &Gemini{
		Client: client,
		Model:  client.GenerativeModel("gemini-pro"),
	}, nil
}

func (g *Gemini) Name() string {
	return "gemini"
}

func (g *Gemini) GenerateSQL(prompt string, schema string) (string, error) {
	ctx := context.Background()
	content := fmt.Sprintf("Given the following DB schema:\n%s\nGenerate a SQL SELECT query for: %s", schema, prompt)

	resp, err := g.Model.GenerateContent(ctx, genai.Text(content))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 {
		return "", errors.New("no response from Gemini")
	}

	return string(resp.Candidates[0].Content.Parts[0].(genai.Text)), nil

}
