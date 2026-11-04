package logcontext

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
)

// NewBundle returns the [flam.IBundle] providing the
// [flam.ILogContextExtractor] of the id and the principal of the request
// of a context in the [log.ContextExtractorGroup] group, collected by the
// [flam.IContextLogger] of the log bundle. The extractor takes the
// [flam.IConfig] from the container.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newExtractor, dig.Group(log.ContextExtractorGroup)).
		Register()
}
