package watchdog

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the watchdog feature, providing
// to the application container a single
// [flam.IWatchdogLoggerAdaptorFactory], created on first use from the
// configuration at [LoggerConfigPath] and the creators of the
// [LoggerCreatorGroup] group, and a single [flam.IKennel], holding the
// processes of the [ProcessGroup] group, configured at
// [ProcessConfigPath], reading the time from the [flam.IClock] of the
// container. The configuration is read once, when they are created. It provides no logger adaptor driver: register it with the
// bundle of [github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers].
//
// The bundle implements [flam.IRunnableBundle]: when the application
// runs, it runs the active processes of the kennel, unless
// [RunConfigPath] holds false, until they all end, closing the kennel on
// an interrupt or a SIGTERM signal, unless [SignalsConfigPath] holds
// false. When the kennel could not be created, as its delays are invalid
// or a process of [ProcessGroup] could not be added, the processes are
// closed, the provider of the kennel fails with that error, and the run
// of the application, or else its close, reports it once. As the run of the application runs the bundles one after the
// other, register this bundle after the others.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the kennel, and so its processes, then the logger
// adaptor factory, even when one fails, returning their errors joined. As
// they are taken from the container, closing an application that never
// used them creates them first, and fails when they cannot be created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time checks that *bundleImpl implements IRunnableBundle and
// IClosableBundle
var (
	_ flam.IRunnableBundle = (*bundleImpl)(nil)
	_ flam.IClosableBundle = (*bundleImpl)(nil)
)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newLoggerFactory).
		QueueProvider(newKennel).
		QueueProvider(b.provideKennel).
		Register()
}

func (b *bundleImpl) Run(
	app flam.IApp,
) error {
	return app.Invoke(b.runKennel)
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeKennel),
		app.Invoke(b.closeLoggerFactory),
	)
}

// provideKennel provides the kennel as a flam.IKennel, failing with the
// error of its creation, if any.
func (b *bundleImpl) provideKennel(
	k *kennel,
) (flam.IKennel, error) {
	if k.err != nil {
		return nil, k.err
	}
	return k, nil
}

func (b *bundleImpl) runKennel(
	config flam.IConfig,
	k *kennel,
) error {
	switch {
	case k.err != nil:
		return k.creationErr()
	case !config.Bool(RunConfigPath, defaultRun):
		return nil
	}
	return k.run(config.Bool(SignalsConfigPath, defaultSignals))
}

func (b *bundleImpl) closeKennel(
	k *kennel,
) error {
	if k.err != nil {
		return k.creationErr()
	}
	return k.close()
}

func (b *bundleImpl) closeLoggerFactory(
	f flam.IWatchdogLoggerAdaptorFactory,
) error {
	return f.Close()
}
