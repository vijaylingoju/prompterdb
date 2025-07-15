// Package cache provides thread-safe caching functionality for database schemas.
package cache

import (
	"sync"
)

var (
	// schemaCache stores the cached schemas with database names as keys
	schemaCache   = make(map[string]string)
	schemaCacheMu sync.RWMutex
)

// CacheSchema stores a schema string for a given database name.
// It's safe for concurrent use by multiple goroutines.
func CacheSchema(dbName string, schema string) {
	if dbName == "" {
		return
	}

	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()
	schemaCache[dbName] = schema
}

// GetCachedSchema returns the cached schema for a database.
// The second return value indicates whether the schema was found in the cache.
func GetCachedSchema(dbName string) (string, bool) {
	if dbName == "" {
		return "", false
	}

	schemaCacheMu.RLock()
	defer schemaCacheMu.RUnlock()
	schema, ok := schemaCache[dbName]
	return schema, ok
}

// GetAllCachedSchemas returns a copy of all cached schemas.
// This is safe for concurrent access and returns a snapshot of the cache.
func GetAllCachedSchemas() map[string]string {
	schemaCacheMu.RLock()
	defer schemaCacheMu.RUnlock()

	// Create a new map to avoid data races
	result := make(map[string]string, len(schemaCache))
	for k, v := range schemaCache {
		result[k] = v
	}

	return result
}

// ClearCache removes all cached schemas.
// This is useful for testing or when you need to force a refresh of all schemas.
func ClearCache() {
	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()

	// Clear the map by creating a new one
	schemaCache = make(map[string]string)
}

// GetCacheSize returns the number of schemas currently in the cache.
// This is primarily useful for testing and monitoring.
func GetCacheSize() int {
	schemaCacheMu.RLock()
	defer schemaCacheMu.RUnlock()
	return len(schemaCache)
}
