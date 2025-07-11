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

type Groq struct {
	APIKey string
	Model  string
}

func NewGroq(apiKey string, model string) (*Groq, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("❌ missing Groq API key")
	}
	if model == "" {
		model = "mixtral-8x7b-32768"
	}
	return &Groq{APIKey: apiKey, Model: model}, nil
}

func (g *Groq) Name() string {
	return "groq"
}

func (g *Groq) GenerateSQL(prompt, schema string) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	// Construct prompt
	message := fmt.Sprintf(`You are a SQL expert. Based on the following schema, write a SELECT SQL query for the user's request.
Schema: %s
User Request: %s`, schema, prompt)

	reqBody := map[string]interface{}{
		"model": g.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant that only responds with valid SQL SELECT statements."},
			{"role": "user", "content": message},
		},
		"temperature": 0.2,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("❌ Groq request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("❌ Groq API error [%d]: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("❌ Failed to parse Groq response: %v", err)
	}

	if len(result.Choices) == 0 {
		return "", errors.New("❌ No response from Groq")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}
