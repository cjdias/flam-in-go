package streams

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/console"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/file"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/otlp"
	rotatingfile "github.com/cjdias/flam-in-go/pkg/flam/log/streams/rotating_file"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// log stream driver, [console.NewBundle], [file.NewBundle],
// [rotatingfile.NewBundle], the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/log/streams/syslog.NewBundle] on
// the platforms providing log/syslog, all but windows and plan9, and
// [otlp.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/log.NewBundle], which provides
// the [flam.ILogStreamFactory]. Do not register any of the driver bundles
// along with this one, as a bundle id can only be registered once.
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
	registerer := bundle.NewRegisterer(app).
		QueueBundle(console.NewBundle()).
		QueueBundle(file.NewBundle()).
		QueueBundle(rotatingfile.NewBundle())
	for _, platformBundle := range platformBundles() {
		registerer = registerer.QueueBundle(platformBundle)
	}
	return registerer.
		QueueBundle(otlp.NewBundle()).
		Register()
}
