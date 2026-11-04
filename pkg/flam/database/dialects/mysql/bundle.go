package mysql

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
)

// NewBundle returns the [flam.IBundle] of the MySQL dialect driver,
// providing its [flam.IDatabaseDialectCreator] in the
// [database.DialectCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the defaults of the dialect fields
// at [DefaultProtocolConfigPath], [DefaultHostConfigPath],
// [DefaultPortConfigPath], [DefaultSchemaConfigPath],
// [DefaultUsernameConfigPath] and [DefaultPasswordConfigPath]. It
// configures no dialect.
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
	for path, value := range map[string]any{
		DefaultProtocolConfigPath: defaultProtocol,
		DefaultHostConfigPath:     defaultHost,
		DefaultPortConfigPath:     defaultPort,
		DefaultSchemaConfigPath:   defaultSchema,
		DefaultUsernameConfigPath: defaultUsername,
		DefaultPasswordConfigPath: defaultPassword,
	} {
		if err := bag.Set(path, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(database.DialectCreatorGroup)).
		Register()
}
