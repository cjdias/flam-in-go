package cache

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// LoadFunc loads the object of a [Loader] from its source, such as a
// database, setting the object, which holds its identity, to its whole
// value. It returns an error wrapping [ErrNotFound] when the object does
// not exist.
type LoadFunc[T any] func(ctx context.Context, obj *T) error

// LoaderOption configures a [Loader], as given to [NewLoader], which
// fails with an error wrapping [ErrInvalidOption] for an invalid value.
type LoaderOption func(o *loaderOptions)

// WithTTL makes the loader store the objects it loads with SetEphemeral,
// expiring after the ttl, which must be positive, instead of with Set,
// never expiring.
func WithTTL(
	ttl time.Duration,
) LoaderOption {
	return func(o *loaderOptions) {
		if ttl <= 0 {
			o.invalid(loaderOptionTTL)
			return
		}
		o.ttl = ttl
	}
}

// WithJitter adds to the ttl of [WithTTL] a random duration between zero
// and the jitter, which must not be negative, so that the objects loaded
// together do not expire together. It does nothing without [WithTTL].
func WithJitter(
	jitter time.Duration,
) LoaderOption {
	return func(o *loaderOptions) {
		if jitter < 0 {
			o.invalid(loaderOptionJitter)
			return
		}
		o.jitter = jitter
	}
}

// WithNegativeTTL makes the loader remember, for the ttl, which must be
// positive, the keys of the objects its load function did not find, so
// that Get fails at once for them, without loading them again. The keys
// are remembered by the loader, in the process, and not by the adaptor:
// removing a value from the cache, or caching it, does not forget them,
// so that the ttl should be short.
func WithNegativeTTL(
	ttl time.Duration,
) LoaderOption {
	return func(o *loaderOptions) {
		if ttl <= 0 {
			o.invalid(loaderOptionNegativeTTL)
			return
		}
		o.negativeTTL = ttl
	}
}

// WithFailClosed makes Get fail with the errors of the adaptor, instead of
// loading the objects it fails to get, and returning the objects it fails
// to store.
func WithFailClosed() LoaderOption {
	return func(o *loaderOptions) {
		o.failClosed = true
	}
}

// WithErrorHandler gives the handler the loader calls with the errors of
// the adaptor it ignores, as it fails open, such as to log them, nil
// calling none.
func WithErrorHandler(
	handler func(ctx context.Context, err error),
) LoaderOption {
	return func(o *loaderOptions) {
		o.onError = handler
	}
}

// WithClock makes the loader read the time of [WithNegativeTTL] from the
// clock, which must not be nil, instead of the system clock.
func WithClock(
	clock flam.IClock,
) LoaderOption {
	return func(o *loaderOptions) {
		if value.IsNil(clock) {
			o.invalid(loaderOptionClock)
			return
		}
		o.now = clock.Now
	}
}

// WithKeyGenerator makes the loader take the keys of the objects from the
// key generator, which must not be nil, instead of from the adaptor, for
// an adaptor that is not a [flam.ICacheKeyedAdaptor]. It must generate
// the keys the adaptor stores the objects under.
func WithKeyGenerator(
	keyGenerator flam.ICacheKeyGenerator,
) LoaderOption {
	return func(o *loaderOptions) {
		if value.IsNil(keyGenerator) {
			o.invalid(loaderOptionKeyGenerator)
			return
		}
		o.keyGenerator = keyGenerator
	}
}

// NewLoader returns the [Loader] of the objects of type T cached by the
// adaptor and loaded by the load function, configured by the options, nil
// ones being skipped. It fails with ErrNilAdaptor or ErrNilLoad when the
// adaptor or the load function is nil, with ErrNotKeyed when the adaptor
// is not a [flam.ICacheKeyedAdaptor] and no [WithKeyGenerator] option is
// given, and with the error of an invalid option.
func NewLoader[T any](
	adaptor flam.ICacheAdaptor,
	load LoadFunc[T],
	opts ...LoaderOption,
) (*Loader[T], error) {
	switch {
	case value.IsNil(adaptor):
		return nil, flam.NewWrappedError(ErrNilAdaptor, loaderNewMessage)
	case load == nil:
		return nil, newErrLoader(ErrNilLoad, loaderNewMessage, adaptor.Id(), "")
	}
	o := loaderOptions{now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.invalidOption != "" {
		return nil, newErrInvalidOption(adaptor.Id(), o.invalidOption)
	}

	l := &Loader[T]{adaptor: adaptor, load: load, opts: o, missing: map[string]time.Time{}}
	keyed, ok := adaptor.(flam.ICacheKeyedAdaptor)
	switch {
	case o.keyGenerator != nil:
		l.key = o.keyGenerator.Generate
	case ok:
		l.key = func(obj any) (string, error) {
			keys, err := keyed.Keys(obj)
			if err != nil {
				return "", err
			}
			return keys[0], nil
		}
	default:
		return nil, newErrLoader(ErrNotKeyed, loaderNewMessage, adaptor.Id(), "")
	}
	return l, nil
}

// Loader reads the objects of type T through a cache adaptor, as the
// cache-aside pattern does: Get sets an object to its cached value, or
// else loads it with the load function, stores it in the cache and sets
// the object to it.
//
// The concurrent Get calls of a loader missing the same key share one
// call of the load function: the first one loads the object, in a
// goroutine of its own, with its context, without its cancellation, and
// the others wait for it, each until its own context is done. Before
// loading, it gets the object from the cache again, so that a Get call
// missing the key just before an other one stored it does not load it
// again. Share one loader per adaptor and type, as the loaders do not
// share their calls.
//
// By default, a loader fails open: an error of the adaptor getting an
// object makes it load the object, and an error storing it makes it
// return the object loaded, the errors being given to the handler of
// [WithErrorHandler], if any, while [WithFailClosed] makes Get fail with
// them. A loader is safe for concurrent use.
type Loader[T any] struct {
	adaptor flam.ICacheAdaptor
	load    LoadFunc[T]
	opts    loaderOptions
	key     func(obj any) (string, error)
	flights singleflight.Group

	mu        sync.Mutex           // guards the fields below
	missing   map[string]time.Time // the expiry of the keys not found
	nextSweep time.Time
}

// compile time check that *Loader implements ICacheLoader
var _ flam.ICacheLoader[struct{}] = (*Loader[struct{}])(nil)

// Get sets the object, holding its identity, to its cached value, or else
// to the value loaded by the load function, storing it in the cache. The
// concurrent calls loading the same key share the value loaded, each
// object being set to a shallow copy of it, so that the maps, slices and
// pointers it holds must not be changed.
//
// Get fails with ErrNilObject when the object is nil, with the error of
// the key generator when its key cannot be generated, with the error of
// the load function, as is, with the error wrapping [ErrNotFound] when the
// key is remembered by [WithNegativeTTL], with the error wrapping
// ErrLoadPanic when the load function panicked, and with the error
// wrapping the error of the context when it is done before the load
// function returns. With [WithFailClosed], it also fails with the errors
// of the adaptor. The object is only changed when Get succeeds.
func (l *Loader[T]) Get(
	ctx context.Context,
	obj *T,
) error {
	id := l.adaptor.Id()
	if obj == nil {
		return newErrLoader(ErrNilObject, loaderGetMessage, id, "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key, err := l.key(obj)
	if err != nil {
		return newErrLoader(err, loaderGetMessage, id, "")
	}
	if l.isMissing(key) {
		return newErrLoader(ErrNotFound, loaderGetMessage, id, key)
	}

	found, err := l.adaptor.Get(ctx, obj)
	switch {
	case err != nil:
		if err := l.fail(ctx, err); err != nil {
			return err
		}
	case found[0] != nil:
		return nil
	}

	seed := *obj // copied here, as obj may change once Get returns
	flight := l.flights.DoChan(key, func() (any, error) {
		return l.fetch(context.WithoutCancel(ctx), key, &seed)
	})
	select {
	case <-ctx.Done():
		return newErrLoader(ctx.Err(), loaderGetMessage, id, key)
	case result := <-flight:
		if result.Err != nil {
			return result.Err
		}
		*obj = *result.Val.(*T)
		return nil
	}
}

// loaderOptions are the options of a Loader.
type loaderOptions struct {
	ttl          time.Duration
	jitter       time.Duration
	negativeTTL  time.Duration
	failClosed   bool
	onError      func(ctx context.Context, err error)
	now          func() time.Time
	keyGenerator flam.ICacheKeyGenerator

	invalidOption string // the name of the first invalid option, if any
}

// invalid records the option as invalid, unless an other one is already.
func (o *loaderOptions) invalid(
	option string,
) {
	if o.invalidOption == "" {
		o.invalidOption = option
	}
}

// Names of the options, held by the ErrorFieldOption field of their
// errors.
const (
	loaderOptionTTL          = "ttl"
	loaderOptionJitter       = "jitter"
	loaderOptionNegativeTTL  = "negative_ttl"
	loaderOptionClock        = "clock"
	loaderOptionKeyGenerator = "key_generator"
)

// Messages of the errors returned by NewLoader and Get.
const (
	loaderNewMessage   = "unable to create cache loader"
	loaderGetMessage   = "unable to get from cache loader"
	loaderPanicMessage = "unable to load cache object"
)

// fetch gets the object of the key from the cache again, or else loads it
// and stores it, returning it, or the error of the load function,
// remembering the keys not found.
func (l *Loader[T]) fetch(
	ctx context.Context,
	key string,
	obj *T,
) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = newErrLoader(ErrLoadPanic, loaderPanicMessage, l.adaptor.Id(), key).
				With(ErrorFieldPanic, r)
		}
	}()

	found, err := l.adaptor.Get(ctx, obj)
	switch {
	case err != nil:
		if err := l.fail(ctx, err); err != nil {
			return nil, err
		}
	case found[0] != nil:
		return obj, nil
	}

	if err := l.load(ctx, obj); err != nil {
		if errors.Is(err, ErrNotFound) && l.opts.negativeTTL > 0 {
			l.remember(key)
		}
		return nil, err
	}
	if err := l.store(ctx, obj); err != nil {
		if err := l.fail(ctx, err); err != nil {
			return nil, err
		}
	}
	return obj, nil
}

// store stores the object in the cache, expiring after the ttl and its
// jitter, if any.
func (l *Loader[T]) store(
	ctx context.Context,
	obj *T,
) error {
	if l.opts.ttl == 0 {
		return l.adaptor.Set(ctx, obj)
	}
	ttl := l.opts.ttl
	if l.opts.jitter > 0 {
		ttl += rand.N(l.opts.jitter + 1)
	}
	return l.adaptor.SetEphemeral(ctx, ttl, obj)
}

// fail returns the error of the adaptor when failing closed, or else
// gives it to the error handler, if any, and returns nil.
func (l *Loader[T]) fail(
	ctx context.Context,
	err error,
) error {
	if l.opts.failClosed {
		return err
	}
	if l.opts.onError != nil {
		l.opts.onError(ctx, err)
	}
	return nil
}

// isMissing reports whether the key is remembered as not found,
// forgetting it once expired.
func (l *Loader[T]) isMissing(
	key string,
) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	expires, ok := l.missing[key]
	if !ok {
		return false
	}
	if l.opts.now().Before(expires) {
		return true
	}
	delete(l.missing, key)
	return false
}

// remember remembers the key as not found for the negative ttl, after
// forgetting the expired keys once per negative ttl.
func (l *Loader[T]) remember(
	key string,
) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.opts.now()
	if !now.Before(l.nextSweep) {
		l.nextSweep = now.Add(l.opts.negativeTTL)
		for k, expires := range l.missing {
			if !now.Before(expires) {
				delete(l.missing, k)
			}
		}
	}
	l.missing[key] = now.Add(l.opts.negativeTTL)
}
