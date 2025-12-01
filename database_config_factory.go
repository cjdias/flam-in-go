package flam

import (
	"io"

	"go.uber.org/dig"
)

type DatabaseConfigFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (DatabaseConfig, error)
	Store(id string, config DatabaseConfig) error
	Remove(id string) error
	RemoveAll() error
}

type databaseConfigFactoryArgs struct {
	dig.In

	Creators      []DatabaseConfigCreator `group:"flam.database.configs.creator"`
	FactoryConfig FactoryConfig
}

func newDatabaseConfigFactory(
	args databaseConfigFactoryArgs,
) (DatabaseConfigFactory, error) {
	var creators []FactoryResourceCreator[DatabaseConfig]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("DatabaseConfig"),
		PathDatabaseConfigs)
}
