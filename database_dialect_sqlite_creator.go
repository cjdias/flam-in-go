package flam

import (
	"fmt"

	"gorm.io/driver/sqlite"
)

type sqliteDatabaseDialectCreator struct {
	config Config
}

var _ DatabaseDialectCreator = (*sqliteDatabaseDialectCreator)(nil)

func newSqliteDatabaseDialectCreator(
	config Config,
) DatabaseDialectCreator {
	return &sqliteDatabaseDialectCreator{
		config: config,
	}
}

func (creator sqliteDatabaseDialectCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == DatabaseDialectDriverSqlite
}

func (creator sqliteDatabaseDialectCreator) Create(
	config Bag,
) (DatabaseDialect, error) {
	dsn := config.String("host", creator.config.String(PathDatabaseDefaultSqliteHost))

	if dsn == "" {
		return nil, newErrInvalidResourceConfig("sqliteDatabaseDialect", "host", config)
	}

	if len(config.Bag("params")) > 0 {
		dsn += "?"
		for key, value := range config.Bag("params") {
			dsn += fmt.Sprintf("&%s=%v", key, value)
		}
	}

	return sqlite.Open(dsn), nil
}
