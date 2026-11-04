//go:build !windows && !plan9

package syslog

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
)

// NewBundle returns the [flam.IBundle] of the syslog stream driver,
// providing its [flam.ILogStreamCreator] in the [log.StreamCreatorGroup]
// group. It is also a [flam.IConfigurableBundle], setting the defaults of
// the stream fields at [DefaultLevelConfigPath],
// [DefaultSerializerConfigPath] and [DefaultFacilityConfigPath]. It
// configures no stream, so that an application writes to syslog only
// when configured to.
//
// The creator takes the [flam.ILogSerializerFactory] from the container:
// register the log bundle to create syslog streams.
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
		DefaultLevelConfigPath:      defaultLevel.String(),
		DefaultSerializerConfigPath: defaultSerializer,
		DefaultFacilityConfigPath:   defaultFacility,
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
		QueueProvider(newCreator, dig.Group(log.StreamCreatorGroup)).
		Register()
}
