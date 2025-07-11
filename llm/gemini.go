package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Gemini struct {
	APIKey string
	Model  string
}

func NewGemini(apiKey string) (*Gemini, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("❌ missing Google API key")
	}

	// Check for model env override
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-pro" // default to v1beta model
	}

	return &Gemini{
		APIKey: apiKey,
		Model:  model,
	}, nil
}

func (g *Gemini) Name() string {
	return "gemini"
}

func (g *Gemini) GenerateSQL(prompt string, schema string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("prompt is empty")
	}

	if g.APIKey == "" || g.Model == "" {
		return "", errors.New("Gemini configuration is incomplete")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s", g.Model, g.APIKey)

	// Construct request body
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{
						"text": fmt.Sprintf(
							"You are a helpful assistant that only returns SQL SELECT queries.\n\nSchema:\n%s\n\nPrompt: %s",
							schema, prompt),
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini API request failed: %v", err)
	}
	defer resp.Body.Close()

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API error [%d]: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Gemini response: %v", err)
	}

	// Validate result
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("Gemini response was empty or malformed")
	}

	query := result.Candidates[0].Content.Parts[0].Text
	return strings.TrimSpace(query), nil
}
