package jwt

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the jwt middleware driver,
// providing its [flam.IRestMiddlewareCreator] in the
// [rest.MiddlewareCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a middleware of id
// [Id] with this driver, and setting the defaults of the middleware
// fields: set its key, such as its [ConfigFieldSecret], and list [Id]
// among the middlewares of a server to use it, before the middlewares
// reading the principal.
//
// The creator takes the [flam.IClock] from the container: register the
// clock bundle. It also takes the [flam.IClientFactory], when the
// container provides one, for the middlewares naming their
// [ConfigFieldJWKSClient].
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
		rest.MiddlewareConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultAlgorithmsConfigPath:          defaultAlgorithms(),
		DefaultIssuerConfigPath:              defaultIssuer,
		DefaultAudienceConfigPath:            defaultAudience,
		DefaultLeewayConfigPath:              defaultLeeway.String(),
		DefaultOptionalConfigPath:            defaultOptional,
		DefaultRequireExpirationConfigPath:   defaultRequireExpiration,
		DefaultJWKSRefreshConfigPath:         defaultJWKSRefresh.String(),
		DefaultJWKSMinRefreshConfigPath:      defaultJWKSMinRefresh.String(),
		DefaultJWKSTimeoutConfigPath:         defaultJWKSTimeout.String(),
		DefaultJWKSClientConfigPath:          defaultJWKSClient,
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
		QueueProvider(newCreator, dig.Group(rest.MiddlewareCreatorGroup)).
		Register()
}
