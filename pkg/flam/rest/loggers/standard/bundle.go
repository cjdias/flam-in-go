package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the standard logger adaptor
// driver, providing its [flam.IRestLoggerAdaptorCreator] in the
// [rest.LoggerCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default an adaptor of id
// [Id] with this driver, making it the default logger adaptor of the
// servers, at [rest.DefaultLoggerConfigPath], and setting the defaults of
// the adaptor fields.
//
// The creator takes the [flam.ILogger] from the container when provided:
// register the log bundle to create standard adaptors.
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
	for path, value := range map[string]any{
		rest.LoggerConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		rest.DefaultLoggerConfigPath:     Id,
		DefaultChannelConfigPath:         defaultChannel,
		DefaultLevelStartConfigPath:      defaultLevelStart.String(),
		DefaultLevelErrorConfigPath:      defaultLevelError.String(),
		DefaultLevelShutdownConfigPath:   defaultLevelShutdown.String(),
		DefaultLevelDoneConfigPath:       defaultLevelDone.String(),
	} {
		if err := bag.Set(path, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(rest.LoggerCreatorGroup)).
		Register()
}
