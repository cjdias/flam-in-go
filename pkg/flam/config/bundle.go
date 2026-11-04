package config

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the configuration feature,
// providing to the application container a single [flam.IConfig], empty
// when created, and the [flam.IConfigParserFactory] and the
// [flam.IConfigSourceFactory], created on first use from the
// configuration at [ParserConfigPath] and [SourceConfigPath] and the
// creators of the [ParserCreatorGroup] and [SourceCreatorGroup] groups.
// It provides no parser nor source driver: register it with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/config/parsers] and
// [github.com/cjdias/flam-in-go/pkg/flam/config/sources], or with the
// driver bundles of their sub packages.
//
// Each factory reads its configuration once, when first used: a parser or
// source configured afterwards, such as by a source declaring other
// sources, is not seen by a factory already created.
//
// The bundle implements [flam.IBootableBundle]: when the application
// boots, it creates every source configured at [SourceConfigPath], in id
// order, with the source factory, and adds it to the [flam.IConfig],
// unless [BootConfigPath] holds false. A source already added to the
// configuration is skipped, and the first source failing to be created or
// added fails the boot.
//
// It then starts reloading the sources added to the configuration, with
// [flam.IConfig] ReloadSources, at the period stored at
// [ReloadPeriodConfigPath], with a recurring trigger of the
// [flam.ITriggerFactory] of the container, and restarts the trigger when
// the period changes. The trigger factory, provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/trigger], is only required while
// the period is positive: the boot fails with ErrMissingTriggerFactory
// otherwise. Once booted, a change of the period that fails to restart the
// trigger, such as a positive period without a trigger factory, leaves the
// sources without reloading, until the period changes again, as nobody
// waits for the observer to report it. A reload, run by the trigger where
// nobody waits for it either, drops its errors: a source failing to reload
// keeps its values, and is reloaded again on the next period.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application stops the reloading, then closes the source factory, then
// the parser factory, even when one fails, returning their errors joined.
// Stopping the reloading does not wait for a reload already under way,
// which may still reload sources while the source factory closes them.
// As the reloader and the factories are taken from the container, closing
// an application that never used them creates them first, and fails when
// they cannot be created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time checks that *bundleImpl implements IBootableBundle and
// IClosableBundle
var (
	_ flam.IBootableBundle = (*bundleImpl)(nil)
	_ flam.IClosableBundle = (*bundleImpl)(nil)
)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newParserFactory).
		QueueProvider(newSourceFactory).
		QueueProvider(newConfig).
		QueueProvider(newBooter).
		QueueProvider(newReloader).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.boot)
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeReloader),
		app.Invoke(b.closeSourceFactory),
		app.Invoke(b.closeParserFactory),
	)
}

// boot adds the configured sources, then starts the reloader.
func (b *bundleImpl) boot(
	booter *booter,
	reloader *reloader,
) error {
	if err := booter.Boot(); err != nil {
		return err
	}
	return reloader.Boot()
}

func (b *bundleImpl) closeReloader(
	r *reloader,
) error {
	return r.Close()
}

func (b *bundleImpl) closeSourceFactory(
	f flam.IConfigSourceFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeParserFactory(
	f flam.IConfigParserFactory,
) error {
	return f.Close()
}
