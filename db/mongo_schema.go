// db/mongo_schema.go
package db

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
)

// Cache and mutex to avoid recomputing schema on every run
var (
	mongoSchemaCache = map[string]string{}
	mongoCacheMutex  sync.RWMutex
)

// GetMongoSchema generates a simple schema-like description from MongoDB collections
func GetMongoSchema(dbName string) (string, error) {
	mongoCacheMutex.RLock()
	if cached, ok := mongoSchemaCache[dbName]; ok {
		mongoCacheMutex.RUnlock()
		return cached, nil
	}
	mongoCacheMutex.RUnlock()

	client, ok := MongoClients[dbName]
	if !ok {
		return "", fmt.Errorf("mongo client not found for: %s", dbName)
	}

	dbConfig, ok := MongoDBs[dbName]
	if !ok {
		return "", fmt.Errorf("mongo DB config not found for: %s", dbName)
	}

	collections, err := client.Database(dbConfig.DBName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		return "", fmt.Errorf("error listing collections: %w", err)
	}

	var schemaBuilder strings.Builder

	for _, coll := range collections {
		schemaBuilder.WriteString(fmt.Sprintf("%s(", coll))

		cur, err := client.Database(dbConfig.DBName).Collection(coll).Find(context.TODO(), bson.M{}, nil)
		if err != nil {
			continue
		}
		defer cur.Close(context.TODO())

		var doc bson.M
		if cur.Next(context.TODO()) {
			if err := cur.Decode(&doc); err == nil {
				fields := []string{}
				for k, v := range doc {
					fields = append(fields, fmt.Sprintf("%s %T", k, v))
				}
				schemaBuilder.WriteString(strings.Join(fields, ", "))
			}
		}
		schemaBuilder.WriteString(")\n")
	}

	// Cache result
	schema := schemaBuilder.String()
	mongoCacheMutex.Lock()
	mongoSchemaCache[dbName] = schema
	mongoCacheMutex.Unlock()

	return schema, nil
}
