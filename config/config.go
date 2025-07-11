// config/config.go
package config

type DBType string

const (
	Postgres DBType = "postgres"
	Mongo    DBType = "mongo"
)

type DBConfig struct {
	Name   string
	Type   DBType
	URI    string
	DBName string // Only used for Mongo
}

var RegisteredDBs = map[string]DBConfig{}

func RegisterDB(cfg DBConfig) {
	RegisteredDBs[cfg.Name] = cfg
}
