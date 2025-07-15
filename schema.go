// Package prompterdb provides functionality for working with database schemas and prompts.
package prompterdb

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/vijaylingoju/prompterdb/cache"
	"github.com/vijaylingoju/prompterdb/config"
	"github.com/vijaylingoju/prompterdb/db"
)

// IntrospectAllSchemas gathers schema info for all registered DBs
// It fetches the schema for each database and caches it for future use.
func IntrospectAllSchemas() error {
	ctx := context.Background()
	return IntrospectAllSchemasWithContext(ctx)
}

// IntrospectAllSchemasWithContext gathers schema info with context support.
// This allows for cancellation and timeouts during schema introspection.
func IntrospectAllSchemasWithContext(ctx context.Context) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(config.RegisteredDBs))

	for name, cfg := range config.RegisteredDBs {
		wg.Add(1)
		go func(name string, cfg config.DBConfig) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				var schema string
				var err error

				switch cfg.Type {
				case config.Postgres:
					schema, err = db.GetPostgresSchema(name)
				case config.Mongo:
					schema, err = db.GetMongoSchema(name)
				default:
					err = fmt.Errorf("unsupported DB type: %s", cfg.Type)
				}

				if err != nil {
					errChan <- fmt.Errorf("failed to introspect %s: %w", name, err)
					return
				}

				mu.Lock()
				cache.CacheSchema(name, schema)
				mu.Unlock()
			}
		}(name, cfg)
	}

	// Close the error channel when all goroutines are done
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Return the first error that occurred, if any
	return <-errChan
}

// GetAllSchemas returns a combined string of all cached schemas.
// The schemas are formatted for use in LLM prompts.
func GetAllSchemas() string {
	// Get all cached schemas
	schemas := cache.GetAllCachedSchemas()

	var builder strings.Builder
	for name, schema := range schemas {
		if schema != "" {
			builder.WriteString(fmt.Sprintf("# Database: %s\n%s\n\n", name, schema))
		}
	}

	return strings.TrimSpace(builder.String())
}

// GetSchema returns the schema for a specific database.
// It returns an empty string if the schema is not found.
func GetSchema(dbName string) string {
	schema, _ := cache.GetCachedSchema(dbName)
	return schema
}

// ClearSchemaCache removes all cached schemas.
func ClearSchemaCache() {
	cache.ClearCache()
}
