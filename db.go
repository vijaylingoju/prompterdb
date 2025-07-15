// prompterdb/db.go
package prompterdb

import (
	"github.com/vijaylingoju/prompterdb/config"
	"github.com/vijaylingoju/prompterdb/db"
)

// ConnectPostgres connects and registers a Postgres database
func ConnectPostgres(name, uri string) error {
	config.RegisterDB(config.DBConfig{
		Name: name,
		Type: config.Postgres,
		URI:  uri,
	})
	return db.ConnectPostgres(name, uri)
}

// ConnectMongo connects and registers a Mongo database
func ConnectMongo(name, uri, dbName string) error {
	config.RegisterDB(config.DBConfig{
		Name:   name,
		Type:   config.Mongo,
		URI:    uri,
		DBName: dbName,
	})
	return db.ConnectMongo(name, uri)
}
