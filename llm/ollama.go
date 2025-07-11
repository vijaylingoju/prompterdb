// llm/ollama.go
package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type Ollama struct {
	Model string
	Host  string
}

func NewOllama(model, host string) *Ollama {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &Ollama{Model: model, Host: host}
}

func (o *Ollama) Name() string {
	return "ollama"
}

func (o *Ollama) GenerateSQL(prompt string, schema string) (string, error) {
	fullPrompt := "Given the following schema:\n" + schema + "\nWrite a SQL SELECT query for: " + prompt

	body := map[string]interface{}{
		"model":  o.Model,
		"prompt": fullPrompt,
		"stream": false,
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(o.Host+"/api/generate", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New("ollama request failed")
	}

	var result struct {
		Response string `json:"response"`
	}
	bs, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bs, &result)

	return result.Response, nil
}
