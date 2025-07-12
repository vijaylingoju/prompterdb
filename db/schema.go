package db

import (
	"context"
	"fmt"
	"strings"
)

// GetPostgresSchema dynamically fetches schema from pgPools
func GetPostgresSchema(dbName string) (string, error) {
	pool, ok := pgPools[dbName]
	if !ok {
		return "", fmt.Errorf("postgres pool not found for: %s", dbName)
	}

	ctx := context.Background()

	// Step 1: Get tables
	tablesRows, err := pool.Query(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
	`)
	if err != nil {
		return "", fmt.Errorf("error fetching tables: %w", err)
	}

	var schema strings.Builder

	for tablesRows.Next() {
		var table string
		if err := tablesRows.Scan(&table); err != nil {
			continue
		}
		schema.WriteString(fmt.Sprintf("%s(", table))

		// Step 2: Get columns
		colsRows, err := pool.Query(ctx, `
			SELECT column_name, data_type
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`, table)
		if err != nil {
			continue
		}

		var cols []string
		for colsRows.Next() {
			var name, dtype string
			if err := colsRows.Scan(&name, &dtype); err != nil {
				continue
			}
			cols = append(cols, fmt.Sprintf("%s %s", name, dtype))
		}
		schema.WriteString(strings.Join(cols, ", ") + ")\n")
	}
	return schema.String(), nil
}
