package migration

import (
	"errors"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the migration feature, providing
// to the application container a single [flam.IMigratorFactory] and
// [flam.IMigrationLoggerAdaptorFactory], created on first use from the
// configuration at [MigratorConfigPath] and [LoggerConfigPath] and the
// creators of the [MigratorCreatorGroup] and [LoggerCreatorGroup] groups.
//
// The bundle provides, in the MigratorCreatorGroup group, the creator of
// the migrators whose configuration has no driver, or the [Driver] driver:
// it creates them, with [NewMigrator], for the migrations of their group
// provided in the [MigrationGroup] container group, with the connection of
// the id at [ConfigFieldConnection], and the logger adaptor of the id at
// [ConfigFieldLogger], or of their defaults, and the [flam.IClock] of the
// container. Register it with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/database], which provides the
// connections.
//
// The bundle implements [flam.IBootableBundle]: when the application
// boots, it applies the pending migrations of every migrator configured at
// [MigratorConfigPath], in id order, when [BootConfigPath] holds true.
// The first migrator failing fails the boot.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the migrator factory, then the logger adaptor
// factory, even when the first fails, returning their errors joined. As
// the factories are taken from the container, closing an application that
// never used them creates them first, and fails when they cannot be
// created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time checks that *bundleImpl implements IBootableBundle and
// IClosableBundle
var (
	_ flam.IBootableBundle = (*bundleImpl)(nil)
	_ flam.IClosableBundle = (*bundleImpl)(nil)
)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newMigratorFactory).
		QueueProvider(newLoggerFactory).
		QueueProvider(newMigratorCreator, dig.Group(MigratorCreatorGroup)).
		QueueProvider(newBooter).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.boot)
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeMigratorFactory),
		app.Invoke(b.closeLoggerFactory),
	)
}

func (b *bundleImpl) boot(
	booter *booter,
) error {
	return booter.Boot()
}

func (b *bundleImpl) closeMigratorFactory(
	f flam.IMigratorFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeLoggerFactory(
	f flam.IMigrationLoggerAdaptorFactory,
) error {
	return f.Close()
}
