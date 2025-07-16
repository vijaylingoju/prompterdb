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
	"time"

	"github.com/vijaylingoju/prompterdb/templates"
)

const (
	defaultTimeout = 60 * time.Second
	geminiAPIURL   = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
)

type Gemini struct {
	*BaseLLM
	APIKey string
	Model  string
	client *http.Client
}

func NewGemini(apiKey, model string) (*Gemini, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, errors.New("google api key is required (set GOOGLE_API_KEY environment variable or pass as parameter)")
		}
	}

	if model == "" {
		model = "gemini-pro" // Default model
	}

	return &Gemini{
		BaseLLM: NewBaseLLM("gemini"),
		APIKey:  apiKey,
		Model:   model,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func (g *Gemini) SetTemplateManager(tm *templates.TemplateManager) {
	g.BaseLLM.templateManager = tm
}

// GenerateQuery generates a query based on the provided request
func (g *Gemini) GenerateQuery(req QueryRequest) (*QueryResponse, error) {
	if g.templateManager == nil {
		return nil, errors.New("template manager not set")
	}

	prompt, err := g.preparePrompt(req)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare prompt: %w", err)
	}

	// Make API request
	text, err := g.makeAPIRequest(prompt)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	// Process and format the response
	return g.processResponse(req, text)
}

// makeAPIRequest sends a request to the Gemini API and returns the response text
func (g *Gemini) makeAPIRequest(prompt string) (string, error) {
	url := fmt.Sprintf(geminiAPIURL+"?key=%s", g.Model, g.APIKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return g.extractTextFromResponse(body)
}

// extractTextFromResponse extracts the text content from the Gemini API response
func (g *Gemini) extractTextFromResponse(body []byte) (string, error) {
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(result.Candidates) == 0 {
		return "", errors.New("no candidates in API response")
	}

	if len(result.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("no text parts in API response")
	}

	text := result.Candidates[0].Content.Parts[0].Text
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "`")
	text = strings.TrimSpace(strings.TrimPrefix(text, "sql"))

	return text, nil
}

// processResponse processes the API response and formats it according to the request
func (g *Gemini) processResponse(req QueryRequest, text string) (*QueryResponse, error) {
	formattedResponse, err := g.formatResponse(req, text, "")
	if err != nil {
		return nil, fmt.Errorf("failed to format response: %w", err)
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal([]byte(formattedResponse), &responseMap); err != nil {
		// If we can't parse the response, return a basic response
		return &QueryResponse{
			Query:       text,
			Explanation: "",
			RawResponse: formattedResponse,
		}, nil
	}

	// Create response with default values
	response := &QueryResponse{
		Query:       text,
		Explanation: "",
		RawResponse: formattedResponse,
	}

	// Update from responseMap if available
	if query, ok := responseMap["query"].(string); ok && query != "" {
		response.Query = query
	}
	if explanation, ok := responseMap["explanation"].(string); ok && explanation != "" {
		response.Explanation = explanation
	}

	return response, nil
}
