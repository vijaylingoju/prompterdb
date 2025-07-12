// db/mongo.go
package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/vijaylingoju/prompterdb/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	MongoClients = map[string]*mongo.Client{}   // Exported for schema access
	MongoDBs     = map[string]config.DBConfig{} // Exported for schema access
	mongoMu      sync.Mutex
)

func ConnectMongo(name, uri string) error {
	mongoMu.Lock()
	defer mongoMu.Unlock()

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("error connecting to mongo %s: %w", name, err)
	}
	MongoClients[name] = client
	MongoDBs[name] = config.RegisteredDBs[name]
	return nil
}

func QueryMongo(name, dbName, collection string, filter bson.M) ([]map[string]interface{}, error) {
	client, ok := MongoClients[name]
	if !ok {
		return nil, fmt.Errorf("mongo client not found for: %s", name)
	}

	coll := client.Database(dbName).Collection(collection)
	cur, err := coll.Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.TODO())

	var results []map[string]interface{}
	if err := cur.All(context.TODO(), &results); err != nil {
		return nil, err
	}
	return results, nil
}
