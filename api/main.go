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

	//load env variables
	postgres_uri := os.Getenv("POSTGRESDB_URI")
	if postgres_uri == "" {
		log.Fatal("POSTGRESDB_URI is required")
	}

	mongo_uri := os.Getenv("MONGODB_URI")
	if mongo_uri == "" {
		log.Fatal("MONGODB_URI is required")
	}

	mongo_name := os.Getenv("MONGODB_NAME")
	if mongo_name == "" {
		log.Fatal("MONGODB_NAME is required")
	}

	postgres_name := os.Getenv("POSTGRESDB_NAME")
	if postgres_name == "" {
		log.Fatal("POSTGRESDB_NAME is required")
	}

	//MONGODB_CONNECTION_NAME
	mongo_connection_name := os.Getenv("MONGODB_CONNECTION_NAME")
	if mongo_connection_name == "" {
		log.Fatal("MONGODB_CONNECTION_NAME is required")
	}

	// STEP 1: Connect your databases (Postgres and/or Mongo)
	err := prompterdb.ConnectPostgres(postgres_name, postgres_uri)
	if err != nil {
		log.Fatalf("Postgres connect failed: %v", err)
	}

	err = prompterdb.ConnectMongo(mongo_connection_name, mongo_uri, mongo_name)
	if err != nil {
		log.Fatalf("Mongo connect failed: %v", err)
	}

	// STEP 2: Introspect all schemas (required for LLM)
	if err := prompterdb.IntrospectAllSchemas(); err != nil {
		log.Fatalf("Schema introspection failed: %v", err)
	}

	// STEP 3: Create LLM client (mock or actual)
	var llmClient llm.LLM
	var llmErr error

	// Try to use OpenAI if API key is available
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		llmClient, llmErr = llm.NewOpenAI(apiKey, os.Getenv("OPENAI_MODEL"))
	} else if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		llmClient, llmErr = llm.NewGemini(apiKey, os.Getenv("GEMINI_MODEL"))
	} else {
		log.Fatal("No LLM API key found. Please set either OPENAI_API_KEY or GEMINI_API_KEY")
	}

	if llmErr != nil {
		log.Fatalf("LLM initialization failed: %v", llmErr)
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
