// db/postgres.go
package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns          = int32(10)
	defaultMinConns          = int32(2)
	defaultMaxConnLifetime   = time.Hour
	defaultMaxConnIdleTime   = time.Minute * 30
	defaultHealthCheckPeriod = time.Minute
)

var (
	// pgPools holds the PostgreSQL connection pools
	pgPools   = make(map[string]*pgxpool.Pool)
	pgPoolsMu sync.RWMutex

	// ErrPoolNotFound is returned when a pool is not found
	ErrPoolNotFound = errors.New("postgres pool not found")
)

// ConnectPostgres establishes a connection to PostgreSQL with the given name and URI
func ConnectPostgres(name, uri string) error {
	if name == "" {
		return errors.New("connection name cannot be empty")
	}

	pgPoolsMu.Lock()
	defer pgPoolsMu.Unlock()

	// Check if pool already exists
	if _, exists := pgPools[name]; exists {
		return nil // Already connected
	}

	// Parse the connection string
	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return fmt.Errorf("failed to parse PostgreSQL connection string: %w", err)
	}

	// Configure connection pool
	config.MaxConns = defaultMaxConns
	config.MinConns = defaultMinConns
	config.MaxConnLifetime = defaultMaxConnLifetime
	config.MaxConnIdleTime = defaultMaxConnIdleTime
	config.HealthCheckPeriod = defaultHealthCheckPeriod

	// Create a new connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL connection pool: %w", err)
	}

	// Verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping PostgreSQL server: %w", err)
	}

	pgPools[name] = pool
	return nil
}

// ClosePostgres closes the PostgreSQL connection with the given name
func ClosePostgres(name string) error {
	pgPoolsMu.Lock()
	defer pgPoolsMu.Unlock()

	pool, exists := pgPools[name]
	if !exists {
		return nil // Already closed
	}

	pool.Close()
	delete(pgPools, name)
	return nil
}

// CloseAllPostgres closes all PostgreSQL connections
func CloseAllPostgres() error {
	pgPoolsMu.Lock()
	defer pgPoolsMu.Unlock()

	var lastErr error
	for name, pool := range pgPools {
		pool.Close()
		delete(pgPools, name)
	}

	return lastErr
}

// QueryPostgres executes a query on the specified PostgreSQL database
func QueryPostgres(name, query string) ([]map[string]interface{}, error) {
	// Input validation
	if name == "" {
		return nil, errors.New("connection name cannot be empty")
	}
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	// Get pool with read lock
	pgPoolsMu.RLock()
	pool, ok := pgPools[name]
	pgPoolsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPoolNotFound, name)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute query
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column information
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	// Process results
	var results []map[string]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("error reading row values: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating query results: %w", err)
	}

	return results, nil
}

// Execute executes a SQL command that doesn't return rows
func Execute(name, command string) (int64, error) {
	if name == "" {
		return 0, errors.New("connection name cannot be empty")
	}
	if command == "" {
		return 0, errors.New("command cannot be empty")
	}

	pgPoolsMu.RLock()
	pool, ok := pgPools[name]
	pgPoolsMu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrPoolNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tag, err := pool.Exec(ctx, command)
	if err != nil {
		return 0, fmt.Errorf("execution failed: %w", err)
	}

	return tag.RowsAffected(), nil
}

// BeginTx starts a transaction
func BeginTx(name string) (pgx.Tx, error) {
	if name == "" {
		return nil, errors.New("connection name cannot be empty")
	}

	pgPoolsMu.RLock()
	pool, ok := pgPools[name]
	pgPoolsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPoolNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return tx, nil
}
