package flam

import (
	"fmt"

	"gorm.io/driver/mysql"
)

type mysqlDatabaseDialectCreator struct {
	config Config
}

var _ DatabaseDialectCreator = (*mysqlDatabaseDialectCreator)(nil)

func newMysqlDatabaseDialectCreator(
	config Config,
) DatabaseDialectCreator {
	return &mysqlDatabaseDialectCreator{
		config: config}
}

func (mysqlDatabaseDialectCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == DatabaseDialectDriverMySql
}

func (creator mysqlDatabaseDialectCreator) Create(
	config Bag,
) (DatabaseDialect, error) {
	username := config.String("username")
	password := config.String("password")
	protocol := config.String("protocol", creator.config.String(PathDatabaseDefaultMySqlProtocol))
	host := config.String("host", creator.config.String(PathDatabaseDefaultMySqlHost))
	port := config.Int("port", creator.config.Int(PathDatabaseDefaultMySqlPort))
	schema := config.String("schema")

	switch {
	case username == "":
		return nil, newErrInvalidResourceConfig("mysqlDatabaseDialect", "username", config)
	case password == "":
		return nil, newErrInvalidResourceConfig("mysqlDatabaseDialect", "password", config)
	case protocol == "":
		return nil, newErrInvalidResourceConfig("mysqlDatabaseDialect", "protocol", config)
	case host == "":
		return nil, newErrInvalidResourceConfig("mysqlDatabaseDialect", "host", config)
	case port == 0:
		return nil, newErrInvalidResourceConfig("mysqlDatabaseDialect", "port", config)
	case schema == "":
		return nil, newErrInvalidResourceConfig("mysqlDatabaseDialect", "schema", config)
	}

	dsn := fmt.Sprintf("%s:%s@%s(%s:%d)/%s", username, password, protocol, host, port, schema)
	if len(config.Bag("params")) > 0 {
		dsn += "?"
		for key, value := range config.Bag("params") {
			dsn += fmt.Sprintf("&%s=%v", key, value)
		}
	}

	return mysql.Open(dsn), nil
}
