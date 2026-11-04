package flam

import (
	"go.uber.org/dig"
)

// IBundle is a feature of an application, registered with
// [IApp.RegisterBundle].
//
// A bundle groups the registrations of a feature: its Register method
// registers, on the application, the providers, invokers, decorators and
// other bundles of the feature, typically with an [IRegisterer]. Its id
// must be unique among the bundles of an application.
type IBundle interface {
	// Id returns the id of the bundle, unique among the bundles of an
	// application.
	Id() string

	// Register registers the providers, invokers, decorators and bundles
	// of the feature on the application.
	Register(app IApp) error
}

// IConfigurableBundle is an [IBundle] giving configuration defaults, called
// by [IApp.RegisterBundle] before its Register method.
type IConfigurableBundle interface {
	// Config sets the configuration defaults of the bundle in the bag.
	//
	// The bag is empty, and the values Config sets in it become
	// configuration defaults: they apply where the application
	// configuration has no value.
	Config(bag IBag) error
}

// IBootableBundle is an [IBundle] with work to do once every bundle is
// registered, such as resolving its values from the container.
type IBootableBundle interface {
	// Boot boots the bundle. [IApp.Boot] calls it once, in registration
	// order, after which no bundle can be registered; a failure stops the
	// boot.
	Boot(app IApp) error
}

// IRunnableBundle is an [IBundle] with work to do when the application
// runs, such as serving requests.
type IRunnableBundle interface {
	// Run runs the bundle. [IApp.Run] calls it, in registration order,
	// once the application booted; a failure stops the run.
	Run(app IApp) error
}

// IClosableBundle is an [IBundle] holding resources to release when the
// application closes.
type IClosableBundle interface {
	// Close releases the resources of the bundle. The Close method of
	// [IApp] calls it once, in reverse registration order, and returns its
	// error, wrapped with the bundle id, joined with those of the other
	// bundles.
	Close(app IApp) error
}

// IRegisterer queues and registers application providers, constructors and
// execution callbacks to be executed on the application container.
//
// Register checks every queued entry before applying any: a nil
// application, a nil bundle, or a provider, invoker or decorator that is not
// a non-nil function is refused. A refused entry stays queued, so a
// registerer holding one refuses every Register: build a new one.
//
// The entries are then applied in order, each at most once. As the
// container cannot take an entry back, a failure leaves the entries before
// it applied, and the failed entry counts as applied: an invoker that
// failed may have run, and a bundle whose Register failed is registered.
// Register can be called again, and resumes after the failed entry, which
// is not applied again.
//
// A bundle is applied with [IApp.RegisterBundle], a provider with
// [IApp.Provide], an invoker with [IApp.Invoke] and a decorator with
// [IApp.Decorate], each with its options. The errors returned are [*Error]
// values wrapping one of the sentinel errors of
// [github.com/cjdias/flam-in-go/pkg/flam/bundle], such as bundle.ErrNilBundle,
// or the error of the application, with the "index" and "kind" context
// fields of the entry, except for the errors of a nil application and of a
// Register called during another one, which concern no entry.
//
// Entries queued while Register applies entries, including by the
// invokers and bundles it applies, are checked and applied by that same
// Register call; entries queued after it are applied by the next one.
// Register called during another Register of the same registerer fails
// instead of waiting for it, as it would wait forever when called from an
// entry it applies.
//
// An IRegisterer is safe for concurrent use.
type IRegisterer interface {
	// QueueBundle queues the bundle, to be registered with
	// [IApp.RegisterBundle], and returns the registerer.
	QueueBundle(bundle IBundle) IRegisterer

	// QueueProvider queues the provider, to be added with [IApp.Provide]
	// with the options, and returns the registerer.
	QueueProvider(provider any, opts ...dig.ProvideOption) IRegisterer

	// QueueInvoker queues the callback, to be called with [IApp.Invoke]
	// with the options, and returns the registerer.
	QueueInvoker(callback any, opts ...dig.InvokeOption) IRegisterer

	// QueueDecorator queues the decorator, to be added with
	// [IApp.Decorate] with the options, and returns the registerer.
	QueueDecorator(decorator any, opts ...dig.DecorateOption) IRegisterer

	// Register checks the queued entries, then applies those not applied
	// yet, in order, as described above.
	Register() error
}
