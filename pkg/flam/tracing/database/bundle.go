package database

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] tracing the statements of the
// database connections, by decorating the creators of the
// [github.com/cjdias/flam-in-go/pkg/flam/database.ConnectionCreatorGroup]
// group, of every driver, so that each connection they create has the
// plugin returned by [NewPlugin], of the
// [go.opentelemetry.io/otel/trace.TracerProvider] of the container,
// added. It requires the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing], providing the tracer
// provider: without it, the connection factory cannot be created.
//
// As the container decorates a value group once, whole, an application
// decorating the creators already cannot register this bundle: add the
// plugin to its connections with Use instead.
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
		QueueDecorator(decorateCreators).
		Register()
}
