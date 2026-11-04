package bundle

import (
	"reflect"
	"sync"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewRegisterer returns an empty [flam.IRegisterer] applying its entries to
// app. A nil app is only reported by Register.
func NewRegisterer(
	app flam.IApp,
) flam.IRegisterer {
	return &registererImpl{app: app}
}

// registererImpl is the [flam.IRegisterer] returned by [NewRegisterer].
type registererImpl struct {
	app flam.IApp

	mu          sync.Mutex
	entries     []registererEntryImpl
	next        int  // index of the first entry not applied yet
	checked     int  // index of the first entry not checked yet
	registering bool // whether a Register call is applying entries
}

// registererEntryImpl is a queued entry: a bundle, or a function with the
// options of its kind.
type registererEntryImpl struct {
	kind         string
	bundle       flam.IBundle
	function     any
	provideOpts  []dig.ProvideOption
	invokeOpts   []dig.InvokeOption
	decorateOpts []dig.DecorateOption
}

// compile time check that *registererImpl implements IRegisterer
var _ flam.IRegisterer = (*registererImpl)(nil)

func (r *registererImpl) QueueBundle(
	bundle flam.IBundle,
) flam.IRegisterer {
	return r.queue(registererEntryImpl{kind: registererKindBundle, bundle: bundle})
}

func (r *registererImpl) QueueProvider(
	provider any,
	opts ...dig.ProvideOption,
) flam.IRegisterer {
	return r.queue(registererEntryImpl{kind: registererKindProvider, function: provider, provideOpts: opts})
}

func (r *registererImpl) QueueInvoker(
	callback any,
	opts ...dig.InvokeOption,
) flam.IRegisterer {
	return r.queue(registererEntryImpl{kind: registererKindInvoker, function: callback, invokeOpts: opts})
}

func (r *registererImpl) QueueDecorator(
	decorator any,
	opts ...dig.DecorateOption,
) flam.IRegisterer {
	return r.queue(registererEntryImpl{kind: registererKindDecorator, function: decorator, decorateOpts: opts})
}

func (r *registererImpl) Register() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if value.IsNil(r.app) {
		return newErrNilApplication(registererRegisterMessage)
	}
	if r.registering {
		return newErrRegistering(registererRegisterMessage)
	}
	r.registering = true
	defer func() { r.registering = false }() // under the lock, even on a panic

	// the lock is released while an entry is applied, as the application
	// runs code, such as invokers and bundles, that may queue new entries;
	// the entries queued meanwhile are checked on each step, so that they
	// are all checked before any of them is applied; an entry, which never
	// changes once queued, is checked once
	for r.next < len(r.entries) {
		for ; r.checked < len(r.entries); r.checked++ {
			if err := r.entries[r.checked].check(r.checked); err != nil {
				return err
			}
		}

		index, entry := r.next, r.entries[r.next]
		r.next++ // applied at most once, even when it fails or panics
		if err := r.applyUnlocked(entry); !value.IsNil(err) {
			return newErrApplication(registererRegisterMessage, err, index, entry.kind)
		}
	}
	return nil
}

// kinds of the queued entries, held by the ErrorFieldKind field
const (
	registererKindBundle    = "bundle"
	registererKindProvider  = "provider"
	registererKindInvoker   = "invoker"
	registererKindDecorator = "decorator"
)

// messages of the errors returned by Register
const (
	registererRegisterMessage = "unable to register"
)

// queue appends an entry and returns the registererImpl, for chaining.
func (r *registererImpl) queue(
	entry registererEntryImpl,
) flam.IRegisterer {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)
	return r
}

// applyUnlocked applies the entry with the lock released, taking it back
// before returning, even when the application panics, so that the
// deferred calls of Register run under the lock.
func (r *registererImpl) applyUnlocked(
	entry registererEntryImpl,
) error {
	r.mu.Unlock()
	defer r.mu.Lock()
	return entry.apply(r.app)
}

// check returns the error refusing the entry at the given index, or nil.
func (e registererEntryImpl) check(
	index int,
) error {
	if e.kind == registererKindBundle {
		if value.IsNil(e.bundle) {
			return newErrNilBundle(registererRegisterMessage, index)
		}
		return nil
	}

	v := reflect.ValueOf(e.function)
	if !v.IsValid() || v.Kind() != reflect.Func || v.IsNil() {
		return newErrInvalidFunction(registererRegisterMessage, index, e.kind)
	}
	return nil
}

// apply applies the entry to the application.
func (e registererEntryImpl) apply(
	app flam.IApp,
) error {
	switch e.kind {
	case registererKindBundle:
		return app.RegisterBundle(e.bundle)
	case registererKindProvider:
		return app.Provide(e.function, e.provideOpts...)
	case registererKindInvoker:
		return app.Invoke(e.function, e.invokeOpts...)
	default:
		return app.Decorate(e.function, e.decorateOpts...)
	}
}
