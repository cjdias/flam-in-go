package flam

import (
	"io"

	"go.uber.org/dig"
)

type MigratorLoggerFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (MigratorLogger, error)
	Store(id string, logger MigratorLogger) error
	Remove(id string) error
	RemoveAll() error
}

type migratorLoggerFactoryArgs struct {
	dig.In

	Creators      []MigratorLoggerCreator `group:"flam.migration.loggers.creator"`
	FactoryConfig FactoryConfig
}

func newMigratorLoggerFactory(
	args migratorLoggerFactoryArgs,
) (MigratorLoggerFactory, error) {
	var creators []FactoryResourceCreator[MigratorLogger]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("MigrationLogger"),
		PathMigratorLoggers)
}
