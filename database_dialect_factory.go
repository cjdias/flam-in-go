package flam

import (
	"io"

	"go.uber.org/dig"
)

type DatabaseDialectFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (DatabaseDialect, error)
	Store(id string, dialect DatabaseDialect) error
	Remove(id string) error
	RemoveAll() error
}

type databaseDialectFactoryArgs struct {
	dig.In

	Creators      []DatabaseDialectCreator `group:"flam.database.dialects.creator"`
	FactoryConfig FactoryConfig
}

func newDatabaseDialectFactory(
	args databaseDialectFactoryArgs,
) (DatabaseDialectFactory, error) {
	var creators []FactoryResourceCreator[DatabaseDialect]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("DatabaseDialect"),
		PathDatabaseDialects)
}
