# PrompterDB - AI-Powered Database Query Library

**⚠️ WARNING: This library is currently in development and testing phase. It may contain inconsistencies and is not yet recommended for production use.**

PrompterDB is a Go library that allows you to interact with databases using natural language queries. It uses AI to convert human-readable prompts into database operations for both SQL and NoSQL databases.

## Features

- Natural language to SQL/MongoDB query conversion
- Support for PostgreSQL and MongoDB databases
- AI-powered query generation and validation
- Schema introspection and caching
- Safe query execution with operation restrictions

## Prerequisites

- Go 1.18 or higher
- PostgreSQL 10+ or MongoDB 4.0+
- Google Gemini API key (for AI query generation)

## Installation

```bash
go get github.com/vijaylingoju/prompterdb
```

## Configuration

Set the following environment variables:

```bash
export GEMINI_API_KEY=your_gemini_api_key
export GEMINI_MODEL=gemini-2.0-flash
export POSTGRES_DSN=postgres://user:password@localhost:5432/dbname
export MONGODB_URI=mongodb://localhost:27017
```

## Usage

### Basic Usage

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
	//dotenv
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run api/main.go \"your natural language prompt here\"")
	}
	prompt := os.Args[1]

	// STEP 1: Connect your databases (Postgres and/or Mongo)
	err := prompterdb.ConnectPostgres("students", "postgres://admin:admin@localhost:5432/students")
	if err != nil {
		log.Fatalf("Postgres connect failed: %v", err)
	}

	err = prompterdb.ConnectMongo("lms", "mongodb://localhost:27017", "lms")
	if err != nil {
		log.Fatalf("Mongo connect failed: %v", err)
	}

	// STEP 2: Introspect all schemas (required for LLM)
	if err := prompterdb.IntrospectAllSchemas(); err != nil {
		log.Fatalf("Schema introspection failed: %v", err)
	}

	// STEP 3: Create LLM client (mock or actual)
	llmClient, err := llm.NewGemini(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"))
	if err != nil {
		log.Fatalf("LLM initialization failed: %v", err)
	}

	// STEP 4: Run the query using your library
	results, err := prompterdb.Ask(prompt, llmClient)
	if err != nil {
		log.Fatalf("Ask failed: %v", err)
	}

	// STEP 5: Print the results
	fmt.Println("✅ Query Results:")
	for i, row := range results {
		fmt.Printf("%d. %v\n", i+1, row)
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
   - Allowed operations: SELECT, INSERT, UPDATE
   - Blocked operations: DROP, TRUNCATE, ALTER, DELETE, CREATE, GRANT, REVOKE

2. **MongoDB Validation**
   - Allowed operations: find, insert, update
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

```go
// SQL Examples
results, err := prompterdb.Ask("Show me the top 10 products by sales", llmClient)
results, err := prompterdb.Ask("Add a new user with email john@example.com", llmClient)

// MongoDB Examples
results, err := prompterdb.Ask("Find all users who signed up in the last 30 days", llmClient)
results, err := prompterdb.Ask("Add a new product with name 'Laptop' and price 1000", llmClient)
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



