package flam

import (
	"io"

	"go.uber.org/dig"
)

type MigratorFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (Migrator, error)
	Store(id string, migrator Migrator) error
	Remove(id string) error
	RemoveAll() error
}

type migratorFactoryArgs struct {
	dig.In

	Creators      []MigratorCreator `group:"flam.migration.migrators.creator"`
	FactoryConfig FactoryConfig
}

func newMigratorFactory(
	args migratorFactoryArgs,
) (MigratorFactory, error) {
	var creators []FactoryResourceCreator[Migrator]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("Migrator"),
		PathMigrators)
}
