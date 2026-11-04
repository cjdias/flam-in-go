package metrics

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc"
)

// NewBundle returns the [flam.IBundle] of the metrics interceptor driver,
// providing its [flam.IGrpcInterceptorCreator] in the
// [grpc.InterceptorCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default an interceptor of id
// [Id] with this driver, and setting the defaults of the interceptor
// fields: list [Id] among the interceptors of a server to use it.
//
// The creator takes the [flam.IClock] from the container, and the
// Prometheus registerer when provided: register the clock bundle, and the
// bundle of [github.com/cjdias/flam-in-go/pkg/flam/metrics] to create
// metrics interceptors.
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
		grpc.InterceptorConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultNamespaceConfigPath:            defaultNamespace,
		DefaultBucketsConfigPath:              defaultBuckets(),
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
		QueueProvider(newCreator, dig.Group(grpc.InterceptorCreatorGroup)).
		Register()
}
