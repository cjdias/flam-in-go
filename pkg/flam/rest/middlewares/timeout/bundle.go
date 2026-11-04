package timeout

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the timeout middleware
// driver, providing its [flam.IRestMiddlewareCreator] in the
// [rest.MiddlewareCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a middleware of id
// [Id] with this driver, and setting the defaults of the middleware
// fields: list [Id] among the middlewares of a server to use it, usually
// last, so that the time taken by the other middlewares is not bounded,
// and that the log middleware logs its answers.
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
		DefaultTimeoutConfigPath:             defaultTimeout.String(),
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
