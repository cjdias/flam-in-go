package database

import (
	"errors"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the database feature, providing
// to the application container a single [flam.IDatabaseConfigFactory],
// [flam.IDatabaseDialectFactory] and [flam.IDatabaseConnectionFactory],
// created on first use from the configuration at [ConfigConfigPath],
// [DialectConfigPath] and [ConnectionConfigPath] and the creators of the
// [ConfigCreatorGroup], [DialectCreatorGroup] and [ConnectionCreatorGroup]
// groups. The configuration is read once, when a factory is created.
//
// The bundle provides, in the ConnectionCreatorGroup group, the creator of
// the connections whose configuration has no driver, or the [Driver]
// driver: it opens them with gorm, with the dialect of the id at
// [ConfigFieldDialect] and the gorm configuration of the id at
// [ConfigFieldConfig], or of their defaults at [DefaultDialectConfigPath]
// and [DefaultConfigConfigPath], along with their read replicas and extra
// sources, retrying as configured with the [flam.IClock] of the
// container, only required by the connections retried. It provides no
// configuration nor dialect driver: register it with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/database/configs] and
// [github.com/cjdias/flam-in-go/pkg/flam/database/dialects].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the connection factory, and so the pools of its
// connections, then the dialect factory, then the configuration factory,
// even when one fails, returning their errors joined. As the factories are
// taken from the container, closing an application that never used them
// creates them first, and fails when they cannot be created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IClosableBundle
var _ flam.IClosableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newConfigFactory).
		QueueProvider(newDialectFactory).
		QueueProvider(newConnectionFactory).
		QueueProvider(newConnectionCreator, dig.Group(ConnectionCreatorGroup)).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeConnectionFactory),
		app.Invoke(b.closeDialectFactory),
		app.Invoke(b.closeConfigFactory),
	)
}

func (b *bundleImpl) closeConnectionFactory(
	f flam.IDatabaseConnectionFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeDialectFactory(
	f flam.IDatabaseDialectFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeConfigFactory(
	f flam.IDatabaseConfigFactory,
) error {
	return f.Close()
}
