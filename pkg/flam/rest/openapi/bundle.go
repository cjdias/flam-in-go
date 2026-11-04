package openapi

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the OpenAPI endpoints,
// providing their [flam.IRestEndpointRegister] in the
// [rest.EndpointGroup] group, documenting the endpoints of the servers of
// its group, to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/rest]. The register reads its
// configuration once, when created, along with the servers.
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
