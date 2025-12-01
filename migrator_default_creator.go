package flam

type defaultMigratorCreator struct {
	config                    Config
	databaseConnectionFactory DatabaseConnectionFactory
	migrationLoggerFactory    MigratorLoggerFactory
	migrationPool             *migrationPool
}

var _ MigratorCreator = (*defaultMigratorCreator)(nil)

func newDefaultMigratorCreator(
	config Config,
	databaseConnectionFactory DatabaseConnectionFactory,
	migrationLoggerFactory MigratorLoggerFactory,
	migrationPool *migrationPool,
) MigratorCreator {
	return &defaultMigratorCreator{
		config:                    config,
		databaseConnectionFactory: databaseConnectionFactory,
		migrationLoggerFactory:    migrationLoggerFactory,
		migrationPool:             migrationPool}
}

func (creator defaultMigratorCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == MigratorDriverDefault
}

func (creator defaultMigratorCreator) Create(
	config Bag,
) (Migrator, error) {
	connectionId := config.String("connection_id", creator.config.String(PathMigratorDefaultConnectionId))
	loggerId := config.String("logger_id", creator.config.String(PathMigratorDefaultLoggerId))
	group := config.String("group")

	switch {
	case connectionId == "":
		return nil, newErrInvalidResourceConfig("defaultMigrator", "connection_id", config)
	case group == "":
		return nil, newErrInvalidResourceConfig("defaultMigrator", "group", config)
	}

	connection, e := creator.databaseConnectionFactory.Get(connectionId)
	if e != nil {
		return nil, e
	}

	var logger MigratorLogger
	if loggerId != "" {
		logger, e = creator.migrationLoggerFactory.Get(loggerId)
		if e != nil {
			return nil, e
		}
	}

	migrations := creator.migrationPool.Group(group)

	return newDefaultMigrator(
		connection,
		logger,
		migrations)
}
