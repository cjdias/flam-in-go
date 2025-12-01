package flam

import "gorm.io/gorm"

type databaseConnectionCreator struct {
	config                 Config
	databaseDialectFactory DatabaseDialectFactory
	databaseConfigFactory  DatabaseConfigFactory
}

func newDatabaseConnectionCreator(
	config Config,
	databaseDialectFactory DatabaseDialectFactory,
	databaseConfigFactory DatabaseConfigFactory,
) *databaseConnectionCreator {
	return &databaseConnectionCreator{
		config:                 config,
		databaseDialectFactory: databaseDialectFactory,
		databaseConfigFactory:  databaseConfigFactory}
}

func (databaseConnectionCreator) Accept(
	_ Bag,
) bool {
	return true
}

func (creator databaseConnectionCreator) Create(
	config Bag,
) (DatabaseConnection, error) {
	dialectId := config.String("dialect_id", creator.config.String(PathDatabaseDefaultDialectId))
	connectionConfigId := config.String("config_id", creator.config.String(PathDatabaseDefaultConfigId))

	switch {
	case dialectId == "":
		return nil, newErrInvalidResourceConfig("DatabaseConnection", "dialect_id", config)
	case connectionConfigId == "":
		return nil, newErrInvalidResourceConfig("DatabaseConnection", "config_id", config)
	}

	dialect, e := creator.databaseDialectFactory.Get(dialectId)
	if e != nil {
		return nil, e
	}

	connectionConfig, e := creator.databaseConfigFactory.Get(connectionConfigId)
	if e != nil {
		return nil, e
	}

	return gorm.Open(dialect, (*gorm.Config)(connectionConfig))
}
