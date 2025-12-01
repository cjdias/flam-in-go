package flam

import (
	"fmt"

	"gorm.io/driver/postgres"
)

type postgresDatabaseDialectCreator struct {
	config Config
}

var _ DatabaseDialectCreator = (*postgresDatabaseDialectCreator)(nil)

func newPostgresDatabaseDialectCreator(
	config Config,
) DatabaseDialectCreator {
	return &postgresDatabaseDialectCreator{
		config: config}
}

func (postgresDatabaseDialectCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == DatabaseDialectDriverPostgres
}

func (creator postgresDatabaseDialectCreator) Create(
	config Bag,
) (DatabaseDialect, error) {
	username := config.String("username")
	password := config.String("password")
	host := config.String("host", creator.config.String(PathDatabaseDefaultPostgresHost))
	port := config.Int("port", creator.config.Int(PathDatabaseDefaultPostgresPort))
	schema := config.String("schema")

	switch {
	case username == "":
		return nil, newErrInvalidResourceConfig("postgresDatabaseDialect", "username", config)
	case password == "":
		return nil, newErrInvalidResourceConfig("postgresDatabaseDialect", "password", config)
	case host == "":
		return nil, newErrInvalidResourceConfig("postgresDatabaseDialect", "host", config)
	case port == 0:
		return nil, newErrInvalidResourceConfig("postgresDatabaseDialect", "port", config)
	case schema == "":
		return nil, newErrInvalidResourceConfig("postgresDatabaseDialect", "schema", config)
	}

	dsn := fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s", username, password, host, port, schema)
	if len(config.Bag("params")) > 0 {
		for key, value := range config.Bag("params") {
			dsn += fmt.Sprintf(" %s=%v", key, value)
		}
	}

	return postgres.Open(dsn), nil
}
