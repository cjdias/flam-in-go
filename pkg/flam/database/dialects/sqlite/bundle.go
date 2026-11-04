package sqlite

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
)

// NewBundle returns the [flam.IBundle] of the SQLite dialect driver,
// providing its [flam.IDatabaseDialectCreator] in the
// [database.DialectCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the default of the path at
// [DefaultPathConfigPath]. It configures no dialect.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IConfigurableBundle
var _ flam.IConfigurableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Config(
	bag flam.IBag,
) error {
	return bag.Set(DefaultPathConfigPath, defaultPath)
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(database.DialectCreatorGroup)).
		Register()
}
