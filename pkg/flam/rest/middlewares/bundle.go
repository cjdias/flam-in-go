package middlewares

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/apikey"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/bodylimit"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/cors"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/jwt"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/ratelimit"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/requestid"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/timeout"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// REST middleware driver, such as [envelope.NewBundle] and
// [log.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/rest.NewBundle], which provides
// the [flam.IRestMiddlewareFactory]. Do not register any of the driver
// bundles along with this one, as a bundle id can only be registered
// once.
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
		QueueBundle(envelope.NewBundle()).
		QueueBundle(log.NewBundle()).
		QueueBundle(jwt.NewBundle()).
		QueueBundle(apikey.NewBundle()).
		QueueBundle(ratelimit.NewBundle()).
		QueueBundle(requestid.NewBundle()).
		QueueBundle(cors.NewBundle()).
		QueueBundle(bodylimit.NewBundle()).
		QueueBundle(timeout.NewBundle()).
		Register()
}
