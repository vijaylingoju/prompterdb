package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vijaylingoju/prompterdb/cache"
)

// SchemaInfo represents database schema information
type SchemaInfo struct {
	TableName    string
	ColumnName   string
	DataType     string
	IsNullable   string
	MaxLength    *int
	Precision    *int
	Scale        *int
	DefaultValue *string
	IsPrimaryKey bool
}

// GetPostgresSchema dynamically fetches schema from pgPools
func GetPostgresSchema(dbName string) (string, error) {
	// Check cache first
	if schema, ok := cache.GetCachedSchema(dbName); ok {
		return schema, nil
	}

	// Get connection pool
	pgPoolsMu.RLock()
	pool, ok := pgPools[dbName]
	pgPoolsMu.RUnlock()

	if !ok {
		return "", fmt.Errorf("%w: %s", ErrPoolNotFound, dbName)
	}

	// Query to get all tables and their columns with more detailed type information
	query := `
        SELECT 
            c.table_name, 
            c.column_name, 
            c.udt_name as data_type,
            c.is_nullable,
            c.character_maximum_length,
            c.numeric_precision,
            c.numeric_scale,
            c.column_default,
            EXISTS (
                SELECT 1 
                FROM information_schema.key_column_usage kcu
                JOIN information_schema.table_constraints tc 
                    ON kcu.constraint_name = tc.constraint_name
                WHERE 
                    tc.constraint_type = 'PRIMARY KEY' 
                    AND kcu.table_name = c.table_name 
                    AND kcu.column_name = c.column_name
                    AND kcu.table_schema = c.table_schema
            ) as is_primary_key
        FROM 
            information_schema.columns c
            JOIN information_schema.tables t 
                ON c.table_name = t.table_name 
                AND c.table_schema = t.table_schema
        WHERE 
            c.table_schema = 'public'
            AND t.table_type = 'BASE TABLE'
        ORDER BY 
            c.table_name, 
            c.ordinal_position
    `

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return "", fmt.Errorf("error querying schema: %w", err)
	}
	defer rows.Close()

	// Build schema string
	var schemaBuilder strings.Builder
	tableSchemas := make(map[string][]SchemaInfo)

	// First pass: collect all schema information
	for rows.Next() {
		var info SchemaInfo
		var defaultVal sql.NullString

		err := rows.Scan(
			&info.TableName,
			&info.ColumnName,
			&info.DataType,
			&info.IsNullable,
			&info.MaxLength,
			&info.Precision,
			&info.Scale,
			&defaultVal,
			&info.IsPrimaryKey,
		)
		if err != nil {
			return "", fmt.Errorf("error scanning schema row: %w", err)
		}

		if defaultVal.Valid {
			info.DefaultValue = &defaultVal.String
		}

		tableSchemas[info.TableName] = append(tableSchemas[info.TableName], info)
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating schema rows: %w", err)
	}

	// Second pass: generate schema string
	for tableName, columns := range tableSchemas {
		if schemaBuilder.Len() > 0 {
			schemaBuilder.WriteString("\n\n")
		}

		schemaBuilder.WriteString(fmt.Sprintf("%s (\n", tableName))

		for i, col := range columns {
			if i > 0 {
				schemaBuilder.WriteString(",\n")
			}

			// Column name
			schemaBuilder.WriteString(fmt.Sprintf("    %s ", col.ColumnName))

			// Data type with precision/scale if applicable
			typeStr := mapPgTypeToGoType(col.DataType, col)
			schemaBuilder.WriteString(typeStr)

			// Nullable
			if col.IsNullable == "NO" {
				schemaBuilder.WriteString(" NOT NULL")
			}

			// Primary key
			if col.IsPrimaryKey {
				schemaBuilder.WriteString(" PRIMARY KEY")
			}

			// Default value
			if col.DefaultValue != nil {
				schemaBuilder.WriteString(fmt.Sprintf(" DEFAULT %s", *col.DefaultValue))
			}
		}

		schemaBuilder.WriteString("\n)")
	}

	schemaStr := schemaBuilder.String()

	// Cache the schema
	cache.CacheSchema(dbName, schemaStr)

	return schemaStr, nil
}

// mapPgTypeToGoType converts PostgreSQL data types to Go types
func mapPgTypeToGoType(pgType string, col SchemaInfo) string {
	switch pgType {
	case "int4", "int8", "int2", "serial", "bigserial":
		return "int64"
	case "float4", "float8", "numeric":
		if col.Precision != nil && col.Scale != nil && *col.Scale > 0 {
			return "float64"
		}
		return "float64"
	case "bool":
		return "bool"
	case "json", "jsonb":
		return "map[string]interface{}"
	case "timestamp", "timestamptz", "date", "time", "timetz":
		return "time.Time"
	case "uuid":
		return "string"
	case "text", "varchar":
		if col.MaxLength != nil && *col.MaxLength > 0 {
			return fmt.Sprintf("string // max length: %d", *col.MaxLength)
		}
		return "string"
	case "bytea":
		return "[]byte"
	default:
		// For array types
		if strings.HasSuffix(pgType, "[]") {
			elementType := mapPgTypeToGoType(strings.TrimSuffix(pgType, "[]"), col)
			return fmt.Sprintf("[]%s", elementType)
		}
		return pgType + " // unknown type"
	}
}

// GetTableSchema returns the schema for a specific table
func GetTableSchema(dbName, tableName string) (string, error) {
	if dbName == "" || tableName == "" {
		return "", errors.New("database name and table name are required")
	}

	schema, err := GetPostgresSchema(dbName)
	if err != nil {
		return "", fmt.Errorf("failed to get schema: %w", err)
	}

	// Parse the schema to find the specific table
	// This is a simplified version - in a real implementation, you'd want to parse the schema properly
	lines := strings.Split(schema, "\n")
	var tableLines []string
	inTargetTable := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), tableName+" (") {
			inTargetTable = true
			tableLines = append(tableLines, line)
		} else if inTargetTable {
			if strings.TrimSpace(line) == ")" {
				tableLines = append(tableLines, line)
				break
			}
			tableLines = append(tableLines, line)
		}
	}

	if len(tableLines) == 0 {
		return "", fmt.Errorf("table %s not found in schema", tableName)
	}

	return strings.Join(tableLines, "\n"), nil
}

// GetColumnType returns the Go type for a specific column
func GetColumnType(dbName, tableName, columnName string) (string, error) {
	if dbName == "" || tableName == "" || columnName == "" {
		return "", errors.New("database name, table name, and column name are required")
	}

	// Get the full schema
	schema, err := GetPostgresSchema(dbName)
	if err != nil {
		return "", fmt.Errorf("failed to get schema: %w", err)
	}

	// Parse the schema to find the specific column
	// This is a simplified version - in a real implementation, you'd want to parse the schema properly
	lines := strings.Split(schema, "\n")
	inTargetTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check if this is the start of the target table
		if strings.HasPrefix(line, tableName+" (") {
			inTargetTable = true
			continue
		}

		// If we're in the target table, look for the column
		if inTargetTable {
			// Check if we've reached the end of the table definition
			if line == ")" {
				break
			}

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}

			// Check if this is the target column
			if strings.HasPrefix(line, columnName+" ") {
				// Extract the type
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					// Join the remaining parts to handle types with spaces (like "timestamp with time zone")
					return strings.Join(parts[1:], " "), nil
				}
			}
		}
	}

	return "", fmt.Errorf("column %s not found in table %s", columnName, tableName)
}
