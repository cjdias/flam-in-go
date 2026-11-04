package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
)

// NewBundle returns the [flam.IBundle] of the standard connection driver,
// providing its [flam.IRedisConnectionCreator] in the
// [redis.ConnectionCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the defaults of the connection
// fields at [DefaultNetworkConfigPath], [DefaultHostConfigPath],
// [DefaultPortConfigPath], [DefaultUsernameConfigPath],
// [DefaultPasswordConfigPath] and [DefaultDatabaseConfigPath]. It
// configures no connection.
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
		DefaultNetworkConfigPath:  defaultNetwork,
		DefaultHostConfigPath:     defaultHost,
		DefaultPortConfigPath:     defaultPort,
		DefaultUsernameConfigPath: defaultUsername,
		DefaultPasswordConfigPath: defaultPassword,
		DefaultDatabaseConfigPath: defaultDatabase,
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
		QueueProvider(newCreator, dig.Group(redis.ConnectionCreatorGroup)).
		Register()
}
