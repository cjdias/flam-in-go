package tracing

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc"
)

// NewBundle returns the [flam.IBundle] of the tracing interceptor driver,
// providing its [flam.IGrpcInterceptorCreator] in the
// [grpc.InterceptorCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default an interceptor of id
// [Id] with this driver: list [Id] among the interceptors of a server to
// use it, first, so that its spans cover the other interceptors too.
//
// The creator takes the [go.opentelemetry.io/otel/trace.TracerProvider]
// and the [go.opentelemetry.io/otel/propagation.TextMapPropagator] from
// the container, when provided: register the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing] to create tracing
// interceptors.
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
	return bag.Set(grpc.InterceptorConfigPath+"."+Id, container.Bag{factory.ConfigFieldDriver: Driver})
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(grpc.InterceptorCreatorGroup)).
		Register()
}
