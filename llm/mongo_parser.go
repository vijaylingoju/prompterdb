package llm

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

// MongoQuery represents a generated MongoDB query structure from LLM
type MongoQuery struct {
	Collection string         `json:"collection"`
	Filter     map[string]any `json:"filter"`
}

// ParseMongoQuery parses JSON string from LLM into MongoQuery struct
func ParseMongoQuery(jsonStr string) (*MongoQuery, error) {
	var mq MongoQuery
	if err := json.Unmarshal([]byte(jsonStr), &mq); err != nil {
		return nil, fmt.Errorf("error parsing LLM Mongo response: %w", err)
	}
	if mq.Collection == "" {
		return nil, fmt.Errorf("missing collection in MongoQuery")
	}
	return &mq, nil
}

// ConvertToBson converts filter into bson.M for Mongo querying
func (mq *MongoQuery) ConvertToBson() (bson.M, error) {
	b, err := json.Marshal(mq.Filter)
	if err != nil {
		return nil, fmt.Errorf("error marshalling Mongo filter: %w", err)
	}
	var filter bson.M
	if err := bson.UnmarshalExtJSON(b, true, &filter); err != nil {
		return nil, fmt.Errorf("error converting filter to bson.M: %w", err)
	}
	return filter, nil
}
