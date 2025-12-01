package flam

import (
	"io"

	"go.uber.org/dig"
)

type WatchdogLoggerFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (WatchdogLogger, error)
	Store(id string, logger WatchdogLogger) error
	Remove(id string) error
	RemoveAll() error
}

type watchddogLoggerFactoryArgs struct {
	dig.In

	Creators      []WatchdogLoggerCreator `group:"flam.watchdog.loggers.creator"`
	FactoryConfig FactoryConfig
}

func newWatchdogLoggerFactory(
	args watchddogLoggerFactoryArgs,
) (WatchdogLoggerFactory, error) {
	var creators []FactoryResourceCreator[WatchdogLogger]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("WatchdogLogger"),
		PathWatchdogLoggers)
}
