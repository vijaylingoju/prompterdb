// db/postgres.go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var pgPools = map[string]*pgxpool.Pool{}

func ConnectPostgres(name, uri string) error {
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		return fmt.Errorf("error connecting to postgres %s: %w", name, err)
	}
	pgPools[name] = pool
	return nil
}

func QueryPostgres(name, query string) ([]map[string]interface{}, error) {
	pool, ok := pgPools[name]
	if !ok {
		return nil, fmt.Errorf("postgres pool not found for: %s", name)
	}

	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := rows.FieldDescriptions()
	result := []map[string]interface{}{}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			row[string(col.Name)] = values[i]
		}
		result = append(result, row)
	}
	return result, nil
}
