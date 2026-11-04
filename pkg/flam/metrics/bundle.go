package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the metrics feature, providing
// to the application container a single Prometheus registry, as a
// [prometheus.Registerer] and as a [prometheus.Gatherer], created on first
// use: it holds the collectors of the Go runtime and of the process,
// unless [GoCollectorConfigPath] or [ProcessCollectorConfigPath] holds
// false, and the collectors of the [CollectorGroup] group. The creation
// fails, with an error wrapping [ErrNilCollector] or [ErrRegister], when
// a collector of the group is nil or cannot be registered.
//
// The registry holds no resource to release, so the bundle closes
// nothing. It provides no collector of the application features, nor any
// REST feature: register them with the bundles of the sub packages.
//
// [prometheus.Registerer]: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Registerer
// [prometheus.Gatherer]: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Gatherer
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
		QueueProvider(newRegistry, dig.As(new(prometheus.Registerer), new(prometheus.Gatherer))).
		Register()
}
