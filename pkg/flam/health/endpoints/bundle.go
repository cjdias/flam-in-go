package endpoints

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the health endpoints, providing
// their [flam.IRestEndpointRegister] in the [rest.EndpointGroup] group,
// serving the probes of the [flam.IHealthRegistry] of the container, to
// be registered along with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/health] and
// [github.com/cjdias/flam-in-go/pkg/flam/rest]. The register reads its
// configuration once, when created, along with the servers, failing with
// ErrInvalidField for an invalid value.
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
