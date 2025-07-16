# PrompterDB - AI-Powered Database Query Library

**⚠️ WARNING: This library is currently in development and testing phase. It may contain inconsistencies and is not yet recommended for production use.**

PrompterDB is a Go library that allows you to interact with databases using natural language queries. It uses AI to convert human-readable prompts into database operations for both SQL and NoSQL databases.

## Features

- Natural language to database query conversion
- Multi-database support:
  - **PostgreSQL**: Full CRUD operations
  - **MongoDB**: Full CRUD operations + Aggregation pipeline
- AI-powered query generation with multiple LLM providers:
  - Google Gemini
  - GROQ
  - OpenAI
- Query validation and safety features:
  - Operation whitelisting
  - Schema validation
  - Parameter sanitization
- Debugging tools for LLM interactions
- Flexible template system for custom prompts

## Prerequisites

- Go 1.18 or higher
- PostgreSQL 10+ or MongoDB 4.0+
- Google Gemini API key (for AI query generation)

## Installation

```bash
go get github.com/vijaylingoju/prompterdb
```

## Configuration

### Environment Variables

#### Database Configuration

```bash
# MongoDB
MONGODB_URI=mongodb://localhost:27017
MONGODB_NAME=your_database_name
MONGODB_CONNECTION_NAME=your_connection_name

# PostgreSQL
POSTGRESDB_URI=postgres://user:password@localhost:5432/your_database
POSTGRESDB_NAME=your_database_name
```

#### LLM Providers (at least one required)

```bash
# Google Gemini
GEMINI_API_KEY=your_gemini_api_key
GEMINI_MODEL=gemini-2.0-flash

# GROQ
GROQ_API_KEY=your_groq_api_key
GROQ_MODEL=llama-3.3-70b-versatile

# OpenAI
OPENAI_API_KEY=your_openai_api_key
OPENAI_MODEL=gpt-4
```

#### Debugging

```bash
# Enable detailed LLM debugging (optional)
DEBUG_LLM=true
```

## Usage

### Basic Example

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/vijaylingoju/prompterdb"
	"github.com/vijaylingoju/prompterdb/llm"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go \"your natural language query\"")
	}
	prompt := os.Args[1]

	// 1. Connect to databases
	err := prompterdb.ConnectPostgres(
		os.Getenv("POSTGRESDB_NAME"),
		os.Getenv("POSTGRESDB_URI"),
	)
	if err != nil {
		log.Fatalf("Postgres connect failed: %v", err)
	}

	err = prompterdb.ConnectMongo(
		os.Getenv("MONGODB_CONNECTION_NAME"),
		os.Getenv("MONGODB_URI"),
		os.Getenv("MONGODB_NAME"),
	)
	if err != nil {
		log.Fatalf("Mongo connect failed: %v", err)
	}

	// 2. Introspect database schemas
	if err := prompterdb.IntrospectAllSchemas(); err != nil {
		log.Fatalf("Schema introspection failed: %v", err)
	}

	// 3. Initialize LLM client (supports Gemini, GROQ, or OpenAI)
	var llmClient llm.LLM
	var llmErr error

	// Try to use OpenAI if API key is available
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		llmClient, llmErr = llm.NewOpenAI(apiKey, os.Getenv("OPENAI_MODEL"))
	} else if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		llmClient, llmErr = llm.NewGemini(apiKey, os.Getenv("GEMINI_MODEL"))
	} else if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		llmClient, llmErr = llm.NewGroq(apiKey, os.Getenv("GROQ_MODEL"))
	} else {
		log.Fatal("No LLM API key found. Please set one of: OPENAI_API_KEY, GEMINI_API_KEY, or GROQ_API_KEY")
	}

	if llmErr != nil {
		log.Fatalf("LLM initialization failed: %v", llmErr)
	}

	// 4. Execute the query
	results, err := prompterdb.Ask(prompt, llmClient)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	// 5. Process results
	fmt.Println("✅ Query Results:")
	for i, row := range results {
		fmt.Printf("%d. %+v\n", i+1, row)
	}
}

```

### Database Connection

The library supports two types of databases:

1. **PostgreSQL**
   - Connection string format: `postgres://user:password@host:port/dbname`
   - Set via `POSTGRES_DSN` environment variable

2. **MongoDB**
   - Connection string format: `mongodb://host:port`
   - Set via `MONGODB_URI` environment variable

### Key Functions

1. `Ask(prompt string, llmClient llm.LLM) ([]map[string]interface{}, error)`
   - Main function to convert natural language to database queries
   - Parameters:
     - `prompt`: Natural language query
     - `llmClient`: Initialized LLM client
   - Returns: Query results as map or error

2. `IntrospectAllSchemas()`
   - Automatically discovers and caches database schemas
   - Must be called before using Ask()

3. `FindMostRelevantMongoCollection(prompt string, dbName string) string`
   - Finds the most relevant MongoDB collection for a given query
   - Parameters:
     - `prompt`: Query text
     - `dbName`: Database name
   - Returns: Collection name

### Validation

The library includes built-in validation for both SQL and MongoDB queries:

1. **SQL Validation**
   - Allowed operations: SELECT, INSERT, UPDATE, AGGREGATE
   - Blocked operations: DROP, TRUNCATE, ALTER, DELETE, CREATE, GRANT, REVOKE

2. **MongoDB Validation**
   - Allowed operations: find, insert, update, aggregate
   - Validates query structure and required fields
   - Ensures proper JSON format

## Error Handling

The library provides detailed error messages for:
- Invalid database connections
- Invalid query formats
- Blocked operations
- Schema validation failures
- Execution errors

## Example Queries

### SQL Examples

```go
// Simple SELECT query
results, err := prompterdb.Ask("get all users with age > 25", llmClient)

// JOIN query
results, err = prompterdb.Ask("get all orders with customer details", llmClient)

// Aggregation
results, err = prompterdb.Ask("get total sales by category", llmClient)
```

### MongoDB Examples

```go
// Simple find query
results, err := prompterdb.Ask("find all products with price > 100", llmClient)

// Aggregation pipeline
results, err = prompterdb.Ask("get average order value by month for the last 6 months", llmClient)

// Complex aggregation with grouping and sorting
results, err = prompterdb.Ask("find top 5 customers by total spending in 2023", llmClient)

// Text search with projection
results, err = prompterdb.Ask("search for 'laptop' in products and return only name and price", llmClient)
```

## Security

- All queries are validated before execution
- Dangerous operations are blocked by default
- Schema caching prevents unnecessary database calls
- Input sanitization is performed on all queries

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For support, please open an issue in the GitHub repository.



