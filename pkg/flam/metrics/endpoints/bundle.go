package endpoints

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the metrics endpoint, providing
// its [flam.IRestEndpointRegister] in the [rest.EndpointGroup] group, so
// that the REST servers of its group serve the metrics of the Prometheus
// gatherer of the container at its path. The group, the path and the id
// of the endpoint are read from [GroupConfigPath], [PathConfigPath] and
// [IdConfigPath] when the register is created.
//
// Register the bundle of [github.com/cjdias/flam-in-go/pkg/flam/metrics],
// providing the gatherer, and the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/rest], with a server of the
// group of the endpoint.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IBundle
var _ flam.IBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newRegister, dig.Group(rest.EndpointGroup)).
		Register()
}
