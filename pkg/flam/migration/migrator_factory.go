package migration

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// migratorFactoryArgs are the dependencies of the migrator factory, taken
// from the container. The group tag repeats MigratorCreatorGroup, as a
// struct tag cannot use a constant.
type migratorFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IMigratorCreator `group:"flam.migration.migrators.creator"`
}

// newMigratorFactory returns the migrator factory creating the migrators
// configured at MigratorConfigPath with the creators of the
// MigratorCreatorGroup group.
func newMigratorFactory(
	args migratorFactoryArgs,
) (flam.IMigratorFactory, error) {
	return factory.New(args.Config.Bag(MigratorConfigPath), args.Creators...)
}
