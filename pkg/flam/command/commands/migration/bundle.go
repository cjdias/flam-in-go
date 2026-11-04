package migration

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/command"
)

// NewBundle returns the [flam.IBundle] of the migrate command, providing
// it, named [Name], in the [command.CommandGroup] group. The command takes
// the [flam.IMigratorFactory] from the container: register it along with
// the bundles of [github.com/cjdias/flam-in-go/pkg/flam/command] and of
// [github.com/cjdias/flam-in-go/pkg/flam/migration].
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
		QueueProvider(newCommand, dig.Group(command.CommandGroup)).
		Register()
}
