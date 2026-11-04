package rest

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
)

// NewBundle returns the [flam.IBundle] of the rest source driver, providing
// its [flam.IConfigSourceCreator] in the [config.SourceCreatorGroup] group,
// and a [flam.IConfigRestClientCreator] creating an [net/http.Client] with
// a timeout of 30 seconds, to replace with a decorator. It is also a
// [flam.IConfigurableBundle], setting the defaults of the source fields at
// [DefaultPriorityConfigPath], [DefaultReloadConfigPath],
// [DefaultTimeoutConfigPath], [DefaultMaxSizeConfigPath] and
// [DefaultParserConfigPath]. It configures no source, as a source needs a
// URI.
//
// The creator takes the [flam.IConfigParserFactory] from the container,
// when provided: register the configuration bundle to create rest sources.
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
		DefaultPriorityConfigPath: defaultPriority,
		DefaultReloadConfigPath:   defaultReload,
		DefaultTimeoutConfigPath:  defaultTimeout,
		DefaultMaxSizeConfigPath:  defaultMaxSize,
		DefaultParserConfigPath:   defaultParser,
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
		QueueProvider(newClientCreator).
		QueueProvider(newCreator, dig.Group(config.SourceCreatorGroup)).
		Register()
}
