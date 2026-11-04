package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo"
)

// NewBundle returns the [flam.IBundle] of the standard connection driver,
// providing its [flam.IMongoConnectionCreator] in the
// [mongo.ConnectionCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the defaults of the connection
// fields at [DefaultURIConfigPath], [DefaultDatabaseConfigPath],
// [DefaultAppNameConfigPath], [DefaultUsernameConfigPath],
// [DefaultPasswordConfigPath] and [DefaultPingConfigPath]. It configures
// no connection.
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
		DefaultURIConfigPath:      defaultURI,
		DefaultDatabaseConfigPath: defaultDatabase,
		DefaultAppNameConfigPath:  defaultAppName,
		DefaultUsernameConfigPath: defaultUsername,
		DefaultPasswordConfigPath: defaultPassword,
		DefaultPingConfigPath:     defaultPing,
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
		QueueProvider(newCreator, dig.Group(mongo.ConnectionCreatorGroup)).
		Register()
}
