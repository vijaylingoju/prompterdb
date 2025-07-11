// db/mongo.go
package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClients = map[string]*mongo.Client{}

func ConnectMongo(name, uri string) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("error connecting to mongo %s: %w", name, err)
	}
	mongoClients[name] = client
	return nil
}

func QueryMongo(name, dbName, collection string, filter bson.M) ([]map[string]interface{}, error) {
	client, ok := mongoClients[name]
	if !ok {
		return nil, fmt.Errorf("Mongo client not found for: %s", name)
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
