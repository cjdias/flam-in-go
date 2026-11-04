package memory

import (
	"container/list"
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewAdaptor returns the [flam.ICacheAdaptor] of the id caching the
// objects in memory, under the keys of the key generator, as the strings
// of the serializer, holding at most maxEntries values, and reading the
// time from the clock. The writes sweep the expired values once per sweep
// period, zero disabling the sweep. It fails with ErrNilKeyGenerator,
// ErrNilSerializer or ErrNilClock when one of them is nil, and with
// ErrInvalidField when maxEntries is not positive or the sweep period is
// negative.
//
// Every call holds the lock of the adaptor while reading or changing the
// values, so that the writes of one call are atomic, while the objects
// are encoded and decoded out of it. A call with a done context fails
// with an error wrapping
// [github.com/cjdias/flam-in-go/pkg/flam/cache.ErrStorage], joined with
// the error of the context, as the redis adaptors do.
func NewAdaptor(
	id string,
	keyGenerator flam.ICacheKeyGenerator,
	serializer flam.ICacheSerializer,
	clock flam.IClock,
	maxEntries int,
	sweepPeriod time.Duration,
) (flam.ICacheAdaptor, error) {
	switch {
	case value.IsNil(keyGenerator):
		return nil, newErrNil(ErrNilKeyGenerator, adaptorNewMessage, id)
	case value.IsNil(serializer):
		return nil, newErrNil(ErrNilSerializer, adaptorNewMessage, id)
	case value.IsNil(clock):
		return nil, newErrNil(ErrNilClock, adaptorNewMessage, id)
	case maxEntries < 1:
		return nil, newErrInvalidField(adaptorNewMessage, id, ConfigFieldMaxEntries)
	case sweepPeriod < 0:
		return nil, newErrInvalidField(adaptorNewMessage, id, ConfigFieldSweepPeriod)
	}
	return &adaptor{
		id:           id,
		keyGenerator: keyGenerator,
		serializer:   serializer,
		clock:        clock,
		maxEntries:   maxEntries,
		sweepPeriod:  sweepPeriod,
		entries:      map[string]*list.Element{},
		order:        list.New(),
		nextSweep:    clock.Now().Add(sweepPeriod),
	}, nil
}

// adaptor is the [flam.ICacheAdaptor] returned by [NewAdaptor].
type adaptor struct {
	id           string
	keyGenerator flam.ICacheKeyGenerator
	serializer   flam.ICacheSerializer
	clock        flam.IClock
	maxEntries   int
	sweepPeriod  time.Duration

	mu        sync.Mutex               // guards the fields below
	entries   map[string]*list.Element // of *entry, by key
	order     *list.List               // of *entry, the most recently used first
	nextSweep time.Time
}

// entry is a value held by an adaptor.
type entry struct {
	key     string
	data    string
	expires time.Time // zero for no expiration
}

func (a *adaptor) Id() string {
	return a.id
}

// Has reports whether the cache holds a value under the key of each
// object, without making it more recently used.
func (a *adaptor) Has(
	ctx context.Context,
	objects ...any,
) ([]bool, error) {
	keys, err := a.keys(adaptorHasMessage, objects)
	if err != nil || len(keys) == 0 {
		return []bool{}, err
	}
	if err := orBackground(ctx).Err(); err != nil {
		return nil, newErrStorage(adaptorHasMessage, a.id, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.clock.Now()
	found := make([]bool, len(keys))
	for i, key := range keys {
		found[i] = a.lookup(key, now) != nil
	}
	return found, nil
}

// Get sets each object to the value held under its key, making it the
// most recently used.
func (a *adaptor) Get(
	ctx context.Context,
	objects ...any,
) ([]any, error) {
	keys, err := a.keys(adaptorGetMessage, objects)
	if err != nil || len(keys) == 0 {
		return []any{}, err
	}
	if err := orBackground(ctx).Err(); err != nil {
		return nil, newErrStorage(adaptorGetMessage, a.id, err)
	}

	// copied under the lock, as a write may replace them once released
	values := make([]string, len(keys))
	hits := make([]bool, len(keys))
	a.mu.Lock()
	now := a.clock.Now()
	for i, key := range keys {
		if elem := a.lookup(key, now); elem != nil {
			a.order.MoveToFront(elem)
			values[i], hits[i] = elem.Value.(*entry).data, true
		}
	}
	a.mu.Unlock()

	// decoded in new values, set once all are, so that a failure changes no
	// object, and a value replaces the whole object, instead of merging in
	fresh := make([]reflect.Value, len(objects))
	for i, data := range values {
		if !hits[i] {
			continue
		}
		target := objects[i]
		if rv := reflect.ValueOf(target); rv.Kind() == reflect.Pointer && !rv.IsNil() {
			fresh[i] = reflect.New(rv.Type().Elem())
			target = fresh[i].Interface()
		}
		if err := a.serializer.Deserialize(data, target); err != nil {
			return nil, newErrObject(adaptorGetMessage, a.id, keys[i], err)
		}
	}

	found := make([]any, len(objects))
	for i, value := range fresh {
		if value.IsValid() {
			reflect.ValueOf(objects[i]).Elem().Set(value.Elem())
			found[i] = objects[i]
		}
	}
	return found, nil
}

// Set stores the objects, with no expiration, making them the most
// recently used.
func (a *adaptor) Set(
	ctx context.Context,
	objects ...any,
) error {
	return a.set(ctx, adaptorSetMessage, 0, objects)
}

// SetEphemeral stores the objects, expiring after the ttl, making them
// the most recently used.
func (a *adaptor) SetEphemeral(
	ctx context.Context,
	ttl time.Duration,
	objects ...any,
) error {
	if ttl <= 0 {
		return newErrNil(ErrInvalidTTL, adaptorSetEphemeralMessage, a.id)
	}
	return a.set(ctx, adaptorSetEphemeralMessage, ttl, objects)
}

func (a *adaptor) Del(
	ctx context.Context,
	objects ...any,
) error {
	keys, err := a.keys(adaptorDelMessage, objects)
	if err != nil || len(keys) == 0 {
		return err
	}
	if err := orBackground(ctx).Err(); err != nil {
		return newErrStorage(adaptorDelMessage, a.id, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, key := range keys {
		if elem, ok := a.entries[key]; ok {
			a.remove(elem)
		}
	}
	return nil
}

// Messages of the errors returned by NewAdaptor and the adaptor methods.
const (
	adaptorNewMessage          = "unable to create memory cache adaptor"
	adaptorHasMessage          = "unable to check memory cache"
	adaptorGetMessage          = "unable to get from memory cache"
	adaptorSetMessage          = "unable to set in memory cache"
	adaptorSetEphemeralMessage = "unable to set ephemeral in memory cache"
	adaptorDelMessage          = "unable to delete from memory cache"
)

// orBackground returns the context, or context.Background when nil.
func orBackground(
	ctx context.Context,
) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// set stores the objects, expiring after the ttl, or never for zero,
// after sweeping the expired values when due, then evicts the least
// recently used values beyond the maximum.
func (a *adaptor) set(
	ctx context.Context,
	message string,
	ttl time.Duration,
	objects []any,
) error {
	keys, data, err := a.encode(message, objects)
	if err != nil || len(keys) == 0 {
		return err
	}
	if err := orBackground(ctx).Err(); err != nil {
		return newErrStorage(message, a.id, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.clock.Now()
	a.sweep(now)
	var expires time.Time
	if ttl > 0 {
		expires = now.Add(ttl)
	}
	for i, key := range keys {
		if elem, ok := a.entries[key]; ok {
			e := elem.Value.(*entry)
			e.data, e.expires = data[i], expires
			a.order.MoveToFront(elem)
			continue
		}
		a.entries[key] = a.order.PushFront(&entry{key: key, data: data[i], expires: expires})
	}
	for a.order.Len() > a.maxEntries {
		a.remove(a.order.Back())
	}
	return nil
}

// lookup returns the element of the value held under the key, or nil
// when none is, removing it when expired. The lock must be held.
func (a *adaptor) lookup(
	key string,
	now time.Time,
) *list.Element {
	elem, ok := a.entries[key]
	if !ok {
		return nil
	}
	if elem.Value.(*entry).expired(now) {
		a.remove(elem)
		return nil
	}
	return elem
}

// sweep removes every expired value, when the sweep is enabled and due.
// The lock must be held.
func (a *adaptor) sweep(
	now time.Time,
) {
	if a.sweepPeriod == 0 || now.Before(a.nextSweep) {
		return
	}
	a.nextSweep = now.Add(a.sweepPeriod)
	for elem := a.order.Front(); elem != nil; {
		next := elem.Next()
		if elem.Value.(*entry).expired(now) {
			a.remove(elem)
		}
		elem = next
	}
}

// remove removes the element of a value. The lock must be held.
func (a *adaptor) remove(
	elem *list.Element,
) {
	delete(a.entries, elem.Value.(*entry).key)
	a.order.Remove(elem)
}

// keys returns the keys of the objects, failing with the error of the
// first one the key generator fails for.
func (a *adaptor) keys(
	message string,
	objects []any,
) ([]string, error) {
	keys := make([]string, len(objects))
	for i, obj := range objects {
		key, err := a.keyGenerator.Generate(obj)
		if err != nil {
			return nil, newErrObject(message, a.id, "", err)
		}
		keys[i] = key
	}
	return keys, nil
}

// encode returns the keys of the objects and their strings, failing with
// the error of the first one the key generator or the serializer fails
// for.
func (a *adaptor) encode(
	message string,
	objects []any,
) ([]string, []string, error) {
	keys, err := a.keys(message, objects)
	if err != nil {
		return nil, nil, err
	}
	data := make([]string, len(keys))
	for i, key := range keys {
		if data[i], err = a.serializer.Serialize(objects[i]); err != nil {
			return nil, nil, newErrObject(message, a.id, key, err)
		}
	}
	return keys, data, nil
}

// expired reports whether the value is expired at now.
func (e *entry) expired(
	now time.Time,
) bool {
	return !e.expires.IsZero() && !now.Before(e.expires)
}
