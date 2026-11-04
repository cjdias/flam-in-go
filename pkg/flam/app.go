package flam

import (
	"io"

	"go.uber.org/dig"
)

// IApp represents the main application interface for managing the
// lifecycle of a flam application, including registration, booting, and
// dependency injection.
//
// An application holds a [dig.Container] in which the bundles register
// their providers. The container provides from the start the
// application, as an IApp. The application returned by
// [github.com/cjdias/flam-in-go/pkg/flam/app.New] also registers the core
// bundles of flam, among which the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/config] providing the
// configuration, an [IConfig]: the application adds to it its initial
// values and the defaults of the bundles, as described by RegisterBundle.
//
// Its lifecycle goes through these steps:
//   - RegisterBundle registers the bundles, which register their
//     providers, invokers and decorators, and may register other bundles;
//   - Boot boots, in registration order, the bundles implementing
//     IBootableBundle, after which no bundle can be registered;
//   - Run runs, in registration order, the bundles implementing
//     IRunnableBundle, booting the application first when needed;
//   - Close closes, in reverse registration order, the bundles
//     implementing IClosableBundle.
//
// The errors returned are [*Error] values wrapping one of the sentinel
// errors of [github.com/cjdias/flam-in-go/pkg/flam/app], or the error of the
// container or of a bundle, with the "bundle" context field holding the
// bundle id when a bundle is concerned.
//
// An IApp is not safe for concurrent use, as the dig.Container it
// relies on is not: set it up and run it from one goroutine. The IConfig
// is safe for concurrent use, and so are the values the bundles
// provide when documented as such.
type IApp interface {
	// Close closes, in reverse registration order, the bundles implementing
	// IClosableBundle, and closes the application. It returns their
	// errors, each wrapped with the bundle id, joined with errors.Join, or
	// nil. Calling Close again is a no-op returning nil.
	io.Closer

	// HasBundle reports whether a bundle with the id is registered.
	HasBundle(id string) bool

	// RegisterBundle registers the bundle: when it implements
	// IConfigurableBundle, its Config method is first called with an
	// empty bag, whose values become configuration defaults, applied where
	// the configuration has no value, once the IConfig is taken from the
	// container; then its Register method is called.
	// A bundle whose Config or Register fails stays registered, as what it
	// registered in the container cannot be taken back, but, registered in
	// part, fails Boot, and so Run, with that error. RegisterBundle fails
	// with app.ErrNilBundle, app.ErrDuplicateBundle, app.ErrBooted,
	// app.ErrClosed, or the bundle error.
	RegisterBundle(bundle IBundle) error

	// Container returns the dependency injection container.
	Container() *dig.Container

	// Invoke calls invoker with its arguments taken from the container.
	// See [dig.Container.Invoke]. It fails with
	// app.ErrInvalidFunction when invoker is not a non-nil function,
	// or app.ErrClosed.
	Invoke(invoker any, opts ...dig.InvokeOption) error

	// Provide adds a constructor to the container. See
	// [dig.Container.Provide]. It fails with app.ErrInvalidFunction
	// when provider is not a non-nil function, or app.ErrClosed.
	Provide(provider any, opts ...dig.ProvideOption) error

	// Decorate adds a decorator to the container. See
	// [dig.Container.Decorate]: a type can be decorated once per scope,
	// so that a value decorated already, such as the IConfig decorated by
	// [github.com/cjdias/flam-in-go/pkg/flam/app.New], can only be
	// decorated again in a child scope. It fails with
	// app.ErrInvalidFunction when decorator is not a non-nil function, or
	// app.ErrClosed.
	Decorate(decorator any, opts ...dig.DecorateOption) error

	// Boot boots, in registration order, the bundles implementing
	// IBootableBundle, stopping at the first failure. It boots them once:
	// later calls boot nothing and return the result of the first one, so a
	// failed boot keeps failing, with the same error. It boots no bundle
	// when a bundle failed to register, failing with that error. It fails
	// with app.ErrBooting when called back by a bundle while booting, as
	// the boot is not over, and with app.ErrClosed once the application is
	// closed, even by a bundle while booting, which stops the boot.
	Boot() error

	// Run boots the application when needed, then runs, in registration
	// order, the bundles implementing IRunnableBundle, stopping at the
	// first failure. When the boot failed, now or on an earlier Boot or Run
	// call, Run returns the boot error and runs no bundle. It fails with
	// app.ErrBooting when called back by a bundle while booting,
	// app.ErrRunning when called back by a bundle while running, and with
	// app.ErrClosed once the application is closed, even by a bundle while
	// running, which stops the run.
	Run() error
}
