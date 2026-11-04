package watchdog

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// loggerFactoryArgs are the dependencies of the logger adaptor factory,
// taken from the container. The group tag repeats LoggerCreatorGroup, as
// a struct tag cannot use a constant.
type loggerFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IWatchdogLoggerAdaptorCreator `group:"flam.watchdog.loggers.creator"`
}

// newLoggerFactory returns the logger adaptor factory creating the
// adaptors configured at LoggerConfigPath with the creators of the
// LoggerCreatorGroup group.
func newLoggerFactory(
	args loggerFactoryArgs,
) (flam.IWatchdogLoggerAdaptorFactory, error) {
	return factory.New(args.Config.Bag(LoggerConfigPath), args.Creators...)
}
