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
	//use dotenv to load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	apiKey := os.Getenv("GROQ_API_KEY")
	model := os.Getenv("GROQ_MODEL")

	if apiKey == "" {
		log.Fatal("GROQ_API_KEY environment variable is not set")
	}
	if model == "" {
		model = "llama-3.3-70b-versatile"
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
	llmClient, err := llm.NewGroq(apiKey, model)
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
