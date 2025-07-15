// engine/router.go
package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/vijaylingoju/prompterdb/config"
	"github.com/vijaylingoju/prompterdb/db"
)

// Common words that shouldn't affect routing decisions
var commonWords = map[string]bool{
	"and": true, "or": true, "the": true, "a": true, "an": true,
	"in": true, "on": true, "at": true, "to": true, "for": true,
	"of": true, "with": true, "by": true, "as": true, "is": true,
}

// Router handles database routing logic
type Router struct {
	mu sync.RWMutex
}

// NewRouter creates a new Router instance
func NewRouter() *Router {
	return &Router{}
}

// RoutePrompt uses the prompt to route to the most appropriate DB
func (r *Router) RoutePrompt(ctx context.Context, prompt string) (config.DBConfig, error) {
	if prompt == "" {
		return config.DBConfig{}, errors.New("prompt cannot be empty")
	}

	// Create a cancellable context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	prompt = strings.ToLower(prompt)
	promptWords := r.extractKeywords(prompt)

	if len(promptWords) == 0 {
		return config.DBConfig{}, errors.New("no meaningful keywords found in prompt")
	}

	// Get all registered DBs
	dbs := config.RegisteredDBs
	if len(dbs) == 0 {
		return config.DBConfig{}, errors.New("no databases registered")
	}

	// Use a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	results := make(chan struct {
		cfg   config.DBConfig
		score int
	}, len(dbs))

	// Process each DB in parallel
	for _, cfg := range dbs {
		wg.Add(1)
		go func(cfg config.DBConfig) {
			defer wg.Done()

			// Check if context is done
			select {
			case <-ctx.Done():
				log.Printf("Skipping DB %s due to context cancellation", cfg.Name)
				return
			default:
			}

			score, err := r.calculateDBMatchScore(ctx, cfg, promptWords)
			if err != nil {
				log.Printf("Error calculating DB match score for %s: %v", cfg.Name, err)
				return
			}

			if score > 0 {
				results <- struct {
					cfg   config.DBConfig
					score int
				}{cfg, score}
			}
		}(cfg)
	}

	// Close the results channel after all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Find the best match
	var (
		bestMatch   config.DBConfig
		highestScore int
		foundMatch  bool
	)

	for result := range results {
		if result.score > highestScore || (result.score == highestScore && !foundMatch) {
			bestMatch = result.cfg
			highestScore = result.score
			foundMatch = true
		}
	}

	if !foundMatch {
		return config.DBConfig{}, fmt.Errorf("no suitable database found for prompt: %s", prompt)
	}

	log.Printf("Selected database %s for prompt with score %d", bestMatch.Name, highestScore)

	return bestMatch, nil
}

// calculateDBMatchScore calculates how well a database matches the given keywords
func (r *Router) calculateDBMatchScore(ctx context.Context, cfg config.DBConfig, keywords []string) (int, error) {
	// Get schema based on DB type
	var schema string
	var err error

	switch cfg.Type {
	case config.Postgres:
		schema, err = db.GetPostgresSchema(cfg.Name)
	case config.Mongo:
		schema, err = db.GetMongoSchema(cfg.Name)
	default:
		return 0, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	if err != nil {
		return 0, fmt.Errorf("error getting schema for %s: %w", cfg.Name, err)
	}

	if schema == "" {
		return 0, errors.New("empty schema")
	}

	schemaLower := strings.ToLower(schema)
	score := 0

	// Check each keyword against the schema
	for _, word := range keywords {
		// Check for exact word matches (with word boundaries)
		if strings.Contains(schemaLower, " "+word+" ") ||
			strings.HasPrefix(schemaLower, word+" ") ||
			strings.HasSuffix(schemaLower, " "+word) {
			// Higher score for exact matches
			score += 3
		} else if strings.Contains(schemaLower, word) {
			// Lower score for partial matches
			score += 1
		}

		// Check for table name matches
		if strings.Contains(schemaLower, " "+word+"(") {
			score += 2 // Bonus for table name matches
		}

		// Check for column name matches
		if strings.Contains(schemaLower, " "+word+" ") {
			score += 1 // Small bonus for column name matches
		}
	}

	return score, nil
}

// extractKeywords extracts meaningful keywords from the prompt
func (r *Router) extractKeywords(prompt string) []string {
	words := strings.Fields(prompt)
	keywords := make([]string, 0, len(words))

	for _, word := range words {
		// Clean the word
		word = strings.TrimSpace(word)
		word = strings.Trim(word, `.,!?;:'"()[]{}`)

		// Skip empty words and common words
		if word == "" || commonWords[word] {
			continue
		}

		// Add the cleaned word to keywords
		keywords = append(keywords, word)
	}

	return keywords
}

// RoutePrompt is a convenience wrapper around Router.RoutePrompt
func RoutePrompt(ctx context.Context, prompt string) (config.DBConfig, error) {
	r := NewRouter()
	return r.RoutePrompt(ctx, prompt)
}
