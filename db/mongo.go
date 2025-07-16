package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vijaylingoju/prompterdb/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	defaultTimeout = 10 * time.Second
	maxPoolSize    = 100
	minPoolSize    = 5
)

var (
	MongoClients = make(map[string]*mongo.Client)
	MongoDBs     = make(map[string]config.DBConfig)
	mongoMu      sync.RWMutex

	ErrClientNotFound    = errors.New("mongo client not found")
	ErrInvalidDBName     = errors.New("invalid database name")
	ErrInvalidCollection = errors.New("invalid collection name")
)

func ConnectMongo(name, uri string) error {
	if name == "" {
		return errors.New("connection name cannot be empty")
	}
	if uri == "" {
		return fmt.Errorf("MongoDB URI cannot be empty for connection: %s", name)
	}

	mongoMu.Lock()
	defer mongoMu.Unlock()

	if _, exists := MongoClients[name]; exists {
		return nil
	}

	clientOptions := options.Client().ApplyURI(uri).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetServerSelectionTimeout(defaultTimeout).
		SetConnectTimeout(defaultTimeout)

	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB %s: %w", name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return fmt.Errorf("failed to ping MongoDB %s: %w", name, err)
	}

	MongoClients[name] = client
	if cfg, exists := config.RegisteredDBs[name]; exists {
		MongoDBs[name] = cfg
	}
	return nil
}

func CloseMongo(name string) error {
	mongoMu.Lock()
	defer mongoMu.Unlock()

	client, exists := MongoClients[name]
	if !exists {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Disconnect(ctx); err != nil {
		return fmt.Errorf("error disconnecting MongoDB client %s: %w", name, err)
	}

	delete(MongoClients, name)
	delete(MongoDBs, name)
	return nil
}

func CloseAllMongo() error {
	mongoMu.Lock()
	defer mongoMu.Unlock()

	var lastErr error
	for name, client := range MongoClients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.Disconnect(ctx); err != nil {
			lastErr = fmt.Errorf("error disconnecting MongoDB client %s: %w", name, err)
		}
		cancel()
		delete(MongoClients, name)
		delete(MongoDBs, name)
	}
	return lastErr
}

func QueryMongo(name, dbName, collection string, filter bson.M) ([]map[string]interface{}, error) {
	if name == "" {
		return nil, errors.New("connection name cannot be empty")
	}
	if dbName == "" {
		return nil, ErrInvalidDBName
	}
	if collection == "" {
		return nil, ErrInvalidCollection
	}

	mongoMu.RLock()
	client, ok := MongoClients[name]
	mongoMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClientNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll := client.Database(dbName).Collection(collection)
	cur, err := coll.Find(ctx, filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("MongoDB find failed: %w", err)
	}
	defer cur.Close(ctx)

	var results []map[string]interface{}
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode MongoDB results: %w", err)
	}
	return results, nil
}

func InsertMongo(name, dbName, collection string, document map[string]interface{}) ([]map[string]interface{}, error) {
	if name == "" || dbName == "" || collection == "" {
		return nil, errors.New("invalid insert parameters")
	}

	mongoMu.RLock()
	client, ok := MongoClients[name]
	mongoMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClientNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Database(dbName).Collection(collection).InsertOne(ctx, document)
	if err != nil {
		return nil, fmt.Errorf("MongoDB insert failed: %w", err)
	}

	return []map[string]interface{}{
		{"status": "success", "operation": "insert"},
	}, nil
}

func UpdateMongo(name, dbName, collection string, filter, update map[string]interface{}) ([]map[string]interface{}, error) {
	if name == "" || dbName == "" || collection == "" {
		return nil, errors.New("invalid update parameters")
	}

	mongoMu.RLock()
	client, ok := MongoClients[name]
	mongoMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClientNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := client.Database(dbName).Collection(collection).UpdateMany(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("MongoDB update failed: %w", err)
	}

	return []map[string]interface{}{
		{"status": "success", "matched": res.MatchedCount, "modified": res.ModifiedCount},
	}, nil
}

func DeleteMongo(name, dbName, collection string, filter map[string]interface{}) ([]map[string]interface{}, error) {
	if name == "" || dbName == "" || collection == "" {
		return nil, errors.New("invalid delete parameters")
	}

	mongoMu.RLock()
	client, ok := MongoClients[name]
	mongoMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClientNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := client.Database(dbName).Collection(collection).DeleteMany(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("MongoDB delete failed: %w", err)
	}

	return []map[string]interface{}{
		{"status": "success", "deleted": res.DeletedCount},
	}, nil
}

func AggregateMongo(name, dbName, collection string, pipeline []bson.M) ([]map[string]interface{}, error) {
	if name == "" || dbName == "" || collection == "" {
		return nil, errors.New("invalid aggregate parameters")
	}

	mongoMu.RLock()
	client, ok := MongoClients[name]
	mongoMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClientNotFound, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := client.Database(dbName).Collection(collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("MongoDB aggregate failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode MongoDB results: %w", err)
	}
	return results, nil
}
