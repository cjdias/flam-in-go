package app

import (
	"errors"
	"reflect"
	"slices"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/clock"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams"
	"github.com/cjdias/flam-in-go/pkg/flam/pubsub"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
)

// New returns a new [flam.IApp] whose configuration starts with a copy of
// values, which may be nil, with the core bundles of flam registered, in
// this order:
//   - [github.com/cjdias/flam-in-go/pkg/flam/clock], providing the
//     [flam.IClock];
//   - [github.com/cjdias/flam-in-go/pkg/flam/pubsub], providing an
//     [flam.IPubSub] of string ids and messages;
//   - [github.com/cjdias/flam-in-go/pkg/flam/trigger], providing the
//     [flam.ITriggerFactory];
//   - [github.com/cjdias/flam-in-go/pkg/flam/disk] and
//     [github.com/cjdias/flam-in-go/pkg/flam/disk/disks], providing the
//     [flam.IDiskFactory] and its local and memory drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/config],
//     [github.com/cjdias/flam-in-go/pkg/flam/config/parsers] and
//     [github.com/cjdias/flam-in-go/pkg/flam/config/sources], providing the
//     [flam.IConfig], its parser and source factories, and their drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/log],
//     [github.com/cjdias/flam-in-go/pkg/flam/log/serializers] and
//     [github.com/cjdias/flam-in-go/pkg/flam/log/streams], providing the
//     [flam.ILogger], its serializer and stream factories, and their
//     drivers.
//
// Registering one of them again fails with ErrDuplicateBundle, and so does
// providing again a value they provide, such as an IClock: decorate it
// instead, once, as the container takes a single decorator per type and
// scope, the IConfig, decorated by the application, excepted.
//
// The application decorates the configuration in the container, adding to
// it, when it is first taken from the container, the defaults collected so
// far, then adding the defaults of the bundles registered afterwards as
// they are. The values are the first defaults, then come those of the core
// bundles, then those of the other bundles, in registration order: the
// first added wins, and the sources and the values stored with Set
// override them all. Values the configuration would refuse, such as a key
// that is not a valid path, are never added and fail Boot, and so Run,
// with the error of the bag merge, as does a core bundle failing to
// register.
func New(
	values flam.IBag,
) flam.IApp {
	a := &appImpl{
		container: dig.New(),
		bundles:   map[string]flam.IBundle{},
	}
	// a new container accepts these
	_ = a.container.Provide(func() flam.IApp { return a })
	_ = a.container.Decorate(a.decorateConfig)

	if !value.IsNil(values) {
		if err := appCheck(values); err != nil {
			a.failed = err
		} else {
			a.pending = append(a.pending, values.Clone())
		}
	}
	for _, b := range []flam.IBundle{
		clock.NewBundle(),
		pubsub.NewBundle(),
		trigger.NewBundle(),
		disk.NewBundle(),
		disks.NewBundle(),
		config.NewBundle(),
		parsers.NewBundle(),
		sources.NewBundle(),
		log.NewBundle(),
		serializers.NewBundle(),
		streams.NewBundle(),
	} {
		_ = a.RegisterBundle(b) // a failure is kept, failing the boot
	}
	return a
}

// appImpl is the [flam.IApp] returned by [New]. As the contract allows,
// it is not safe for concurrent use, and takes no lock: a bundle may call
// it back from its Config, Register, Boot, Run or Close method.
type appImpl struct {
	container *dig.Container
	failed    error // the values refused by New and the registration failures, failing the boot

	config  flam.IConfig // once taken from the container, or nil
	pending []flam.IBag  // the defaults to add to the config, until taken

	order   []flam.IBundle          // in registration order
	bundles map[string]flam.IBundle // by id

	booting bool
	booted  bool
	running bool
	bootErr error // the result of the boot, once booted
	closing bool
	closed  bool
}

// compile time check that *appImpl implements IApp
var _ flam.IApp = (*appImpl)(nil)

func (a *appImpl) Close() error {
	if a.closing {
		return nil
	}
	a.closing = true

	var errs []error
	for _, b := range slices.Backward(a.order) {
		closable, ok := b.(flam.IClosableBundle)
		if !ok {
			continue
		}
		if err := closable.Close(a); err != nil {
			errs = append(errs, newErrBundle(appCloseMessage, b.Id(), err))
		}
	}
	a.closed = true
	return errors.Join(errs...)
}

func (a *appImpl) HasBundle(
	id string,
) bool {
	_, ok := a.bundles[id]
	return ok
}

func (a *appImpl) RegisterBundle(
	bundle flam.IBundle,
) error {
	switch {
	case value.IsNil(bundle):
		return newErrNilBundle(appRegisterMessage)
	case a.closing:
		return newErrClosed(appRegisterMessage)
	case a.booted:
		return newErrBooted(appRegisterMessage)
	}
	id := bundle.Id()
	if a.HasBundle(id) {
		return newErrDuplicateBundle(appRegisterMessage, id)
	}

	// registered first: what it registers cannot be taken back on failure
	a.order = append(a.order, bundle)
	a.bundles[id] = bundle

	if err := a.register(bundle); err != nil {
		err = newErrBundle(appRegisterMessage, id, err)
		a.failed = errors.Join(a.failed, err) // registered in part: cannot boot
		return err
	}
	return nil
}

func (a *appImpl) Container() *dig.Container {
	return a.container
}

func (a *appImpl) Invoke(
	invoker any,
	opts ...dig.InvokeOption,
) error {
	if err := a.check(appInvokeMessage, invoker); err != nil {
		return err
	}
	if err := a.container.Invoke(invoker, opts...); err != nil {
		return newErrWrapped(appInvokeMessage, err)
	}
	return nil
}

func (a *appImpl) Provide(
	provider any,
	opts ...dig.ProvideOption,
) error {
	if err := a.check(appProvideMessage, provider); err != nil {
		return err
	}
	if err := a.container.Provide(provider, opts...); err != nil {
		return newErrWrapped(appProvideMessage, err)
	}
	return nil
}

func (a *appImpl) Decorate(
	decorator any,
	opts ...dig.DecorateOption,
) error {
	if err := a.check(appDecorateMessage, decorator); err != nil {
		return err
	}
	if err := a.container.Decorate(decorator, opts...); err != nil {
		return newErrWrapped(appDecorateMessage, err)
	}
	return nil
}

func (a *appImpl) Boot() error {
	if a.closing {
		return newErrClosed(appBootMessage)
	}
	if a.booting { // called back by a bundle: the boot is not over
		return newErrBooting(appBootMessage)
	}
	if a.booted {
		return a.bootErr
	}
	a.booted = true
	a.booting = true
	defer func() { a.booting = false }()

	if a.failed != nil {
		a.bootErr = newErrWrapped(appBootMessage, a.failed)
		return a.bootErr
	}
	// the bundles cannot change: registering fails once booted
	for _, b := range a.order {
		if a.closing { // by a bundle
			a.bootErr = newErrClosed(appBootMessage)
			return a.bootErr
		}
		bootable, ok := b.(flam.IBootableBundle)
		if !ok {
			continue
		}
		if err := bootable.Boot(a); err != nil {
			a.bootErr = newErrBundle(appBootMessage, b.Id(), err)
			return a.bootErr
		}
	}
	return nil
}

func (a *appImpl) Run() error {
	if a.closing {
		return newErrClosed(appRunMessage)
	}
	if a.booting {
		return newErrBooting(appRunMessage)
	}
	if a.running {
		return newErrRunning(appRunMessage)
	}
	if err := a.Boot(); err != nil {
		return err
	}
	a.running = true
	defer func() { a.running = false }()

	for _, b := range a.order {
		if a.closing { // by a bundle
			return newErrClosed(appRunMessage)
		}
		runnable, ok := b.(flam.IRunnableBundle)
		if !ok {
			continue
		}
		if err := runnable.Run(a); err != nil {
			return newErrBundle(appRunMessage, b.Id(), err)
		}
	}
	return nil
}

// Messages of the errors returned by the application methods.
const (
	appRegisterMessage = "unable to register application bundle"
	appInvokeMessage   = "unable to invoke application function"
	appProvideMessage  = "unable to provide application function"
	appDecorateMessage = "unable to decorate application function"
	appBootMessage     = "unable to boot application"
	appRunMessage      = "unable to run application"
	appCloseMessage    = "unable to close application bundle"
)

// check fails once the application is closed, or when fn is not a non-nil
// function, which the container does not always refuse.
func (a *appImpl) check(
	message string,
	fn any,
) error {
	if a.closed {
		return newErrClosed(message)
	}
	if fn == nil || reflect.TypeOf(fn).Kind() != reflect.Func || value.IsNil(fn) {
		return newErrInvalidFunction(message)
	}
	return nil
}

// register configures the bundle, adding its defaults, then registers it.
func (a *appImpl) register(
	bundle flam.IBundle,
) error {
	if configurable, ok := bundle.(flam.IConfigurableBundle); ok {
		defaults := container.Bag{}
		if err := configurable.Config(defaults); err != nil {
			return err
		}
		if err := a.addDefaults(defaults); err != nil {
			return err
		}
	}
	return bundle.Register(a)
}

// addDefaults adds the defaults to the configuration once taken from the
// container, or keeps them until then, failing when the configuration
// would refuse them.
func (a *appImpl) addDefaults(
	defaults flam.IBag,
) error {
	if a.config != nil {
		return a.config.AddDefaults(defaults)
	}
	if err := appCheck(defaults); err != nil {
		return err
	}
	a.pending = append(a.pending, defaults.Clone()) // the bundle may keep it
	return nil
}

// decorateConfig is the decorator of the configuration provided to the
// container, adding the defaults kept so far, in order. They were checked
// when kept, so the configuration accepts them.
func (a *appImpl) decorateConfig(
	config flam.IConfig,
) flam.IConfig {
	for _, defaults := range a.pending {
		_ = config.AddDefaults(defaults) // checked by addDefaults
	}
	a.config, a.pending = config, nil
	return config
}

// appCheck returns the error of the configuration refusing the values, as
// AddDefaults merges them, such as for a key that is not a valid path.
func appCheck(
	values flam.IBag,
) error {
	_, err := container.Bag{}.Merge("", values)
	return err
}
