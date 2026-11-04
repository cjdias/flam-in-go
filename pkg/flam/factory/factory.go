package factory

import (
	"errors"
	"io"
	"maps"
	"slices"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// New returns an [flam.IFactory] creating its resources from config, a
// bag holding one resource configuration bag (a Bag or a map[string]any)
// per resource id, with the given creators, tried in order. Entries of
// config holding any other kind of value are not resource configurations
// and are ignored. The configuration is copied, so later changes to config
// do not affect the factory. The [ConfigFieldId] field of a
// resource configuration is reserved: the factory sets it to the resource
// id in the copies it gives to the creators.
//
// config may be any flam.IBag: a nil one, or one holding a nil pointer,
// holds no configuration, and its
// content is read through its Get("") method, which must return the whole
// bag as a Bag or a map[string]any, as a container.Bag does; any other
// result holds no configuration either.
//
// New returns a [*flam.Error] wrapping [ErrNilCreator] when one
// of the creators is nil. A factory without creators is valid, but can only
// hold resources given to Store.
func New[R flam.IResource](
	config flam.IBag,
	creators ...flam.IResourceCreator[R],
) (flam.IFactory[R], error) {
	for _, creator := range creators {
		if value.IsNil(creator) {
			return nil, newErrNilCreator(factoryNewMessage)
		}
	}

	resources := map[string]flam.IBag{}
	if !value.IsNil(config) {
		entries, _ := factoryBagAs(config.Get(""))
		for id, entry := range entries {
			if bag, isBag := factoryBagAs(entry); isBag {
				if bag == nil {
					bag = container.Bag{} // an empty slot is an empty configuration
				}
				resources[id] = bag.Clone()
			}
		}
	}

	return &factoryImpl[R]{
		config:   resources,
		creators: slices.Clone(creators),
		stored:   map[string]R{},
		pending:  map[string]*factoryCallImpl[R]{},
	}, nil
}

// factoryImpl is the [flam.IFactory] returned by [New].
type factoryImpl[R flam.IResource] struct {
	config   map[string]flam.IBag // immutable after construction
	creators []flam.IResourceCreator[R]

	mu      sync.Mutex
	closed  bool
	stored  map[string]R
	pending map[string]*factoryCallImpl[R] // resources being created by Get
}

// factoryCallImpl is a resource creation by Get, shared with the concurrent
// Get calls for the same id, which wait for done.
type factoryCallImpl[R flam.IResource] struct {
	done     chan struct{}
	resource R
	err      error
}

// compile time check that *factoryImpl implements IFactory
var _ flam.IFactory[flam.IResource] = (*factoryImpl[flam.IResource])(nil)

func (f *factoryImpl[R]) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	stored := f.stored
	f.stored = map[string]R{}
	f.mu.Unlock()

	return factoryRelease(stored)
}

func (f *factoryImpl[R]) Available() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}
	return factorySortedIDs(f.config)
}

func (f *factoryImpl[R]) Stored() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}
	return factorySortedIDs(f.stored)
}

func (f *factoryImpl[R]) Has(
	id string,
) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return false
	}
	_, stored := f.stored[id]
	_, available := f.config[id]
	return stored || available
}

func (f *factoryImpl[R]) Lookup(
	id string,
) (R, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	resource, ok := f.stored[id] // empty once closed
	return resource, ok
}

func (f *factoryImpl[R]) Get(
	id string,
) (resource R, err error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		var zero R
		return zero, newErrClosed(factoryGetMessage).With(ErrorFieldID, id)
	}
	if stored, ok := f.stored[id]; ok {
		f.mu.Unlock()
		return stored, nil
	}
	if call, ok := f.pending[id]; ok {
		f.mu.Unlock()
		<-call.done
		return call.resource, call.err
	}
	call := &factoryCallImpl[R]{done: make(chan struct{})}
	f.pending[id] = call
	f.mu.Unlock()

	// finish the call even when the creator panics, so that the concurrent
	// Get calls waiting for it do not wait forever; the results are set
	// here, as storing may turn a created resource into an error
	defer func() {
		f.mu.Lock()
		delete(f.pending, id)
		switch {
		case call.err != nil:
		case f.closed:
			_ = factoryCloseResource(call.resource)
			var zero R
			call.resource = zero
			call.err = newErrClosed(factoryGetMessage).With(ErrorFieldID, id)
		default:
			f.stored[id] = call.resource
		}
		f.mu.Unlock()
		close(call.done)
		resource, err = call.resource, call.err
	}()

	call.err = newErrCreatorPanic(factoryGetMessage, id) // unless replaced
	call.resource, call.err = f.Generate(id)
	return call.resource, call.err
}

func (f *factoryImpl[R]) Store(
	id string,
	resource R,
) error {
	f.mu.Lock()
	closed := f.closed
	f.mu.Unlock()
	if closed {
		return newErrClosed(factoryStoreMessage).With(ErrorFieldID, id)
	}
	if value.IsNil(resource) {
		return newErrNilResource(factoryStoreMessage, id)
	}
	if resource.Id() != id {
		return newErrIDMismatch(factoryStoreMessage, id, resource.Id())
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed { // meanwhile
		return newErrClosed(factoryStoreMessage).With(ErrorFieldID, id)
	}
	_, stored := f.stored[id]
	_, pending := f.pending[id]
	if stored || pending {
		return newErrDuplicateID(factoryStoreMessage, id)
	}
	f.stored[id] = resource
	return nil
}

func (f *factoryImpl[R]) Generate(
	id string,
) (R, error) {
	var zero R

	f.mu.Lock()
	closed := f.closed
	f.mu.Unlock()
	if closed {
		return zero, newErrClosed(factoryGenerateMessage).With(ErrorFieldID, id)
	}

	config, ok := f.config[id]
	if !ok {
		return zero, newErrUnknownID(factoryGenerateMessage, id)
	}

	for _, creator := range f.creators {
		if !creator.Accept(factoryCreatorConfig(config, id)) {
			continue
		}

		resource, err := creator.Create(factoryCreatorConfig(config, id))
		switch {
		case err != nil:
			return zero, newErrResource(factoryGenerateMessage, err, id)
		case value.IsNil(resource):
			return zero, newErrNilResource(factoryGenerateMessage, id)
		case resource.Id() != id:
			_ = factoryCloseResource(resource) // unusable, and owned by nobody
			return zero, newErrIDMismatch(factoryGenerateMessage, id, resource.Id())
		}
		return resource, nil
	}
	return zero, newErrNoCreator(factoryGenerateMessage, id)
}

func (f *factoryImpl[R]) GenerateAll() error {
	for _, id := range f.Available() {
		if _, err := f.Get(id); err != nil {
			// closed meanwhile: concerns no resource; the error is not
			// matched, as it may be the one of a closed factory a creator
			// uses, which concerns the resource
			f.mu.Lock()
			closed := f.closed
			f.mu.Unlock()
			if closed {
				return newErrClosed(factoryGenerateAllMessage)
			}
			return err
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return newErrClosed(factoryGenerateAllMessage)
	}
	return nil
}

func (f *factoryImpl[R]) Remove(
	id string,
) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return newErrClosed(factoryRemoveMessage).With(ErrorFieldID, id)
	}
	resource, ok := f.stored[id]
	if !ok {
		f.mu.Unlock()
		return newErrUnknownID(factoryRemoveMessage, id)
	}
	delete(f.stored, id)
	f.mu.Unlock()

	if err := factoryCloseResource(resource); err != nil {
		return newErrResource(factoryRemoveMessage, err, id)
	}
	return nil
}

func (f *factoryImpl[R]) RemoveAll() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return newErrClosed(factoryRemoveAllMessage)
	}
	stored := f.stored
	f.stored = map[string]R{}
	f.mu.Unlock()

	return factoryRelease(stored)
}

// messages of the errors returned by New and the methods
const (
	factoryNewMessage         = "unable to create factory"
	factoryGetMessage         = "unable to get resource"
	factoryStoreMessage       = "unable to store resource"
	factoryGenerateMessage    = "unable to generate resource"
	factoryGenerateAllMessage = "unable to generate resources"
	factoryRemoveMessage      = "unable to remove resource"
	factoryRemoveAllMessage   = "unable to remove resources"
)

// factoryCreatorConfig returns the copy of a resource configuration given to
// a creator method: a deep copy holding the resource id in the
// ConfigFieldId field.
func factoryCreatorConfig(
	config flam.IBag,
	id string,
) flam.IBag {
	result := config.Clone()
	_ = result.Set(ConfigFieldId, id) // valid path on a non-nil bag
	return result
}

// factoryRelease closes, in id order, the given removed resources
// implementing io.Closer, and returns their Close errors joined, or nil.
func factoryRelease[R flam.IResource](
	resources map[string]R,
) error {
	var errs []error
	for _, id := range slices.Sorted(maps.Keys(resources)) {
		if err := factoryCloseResource(resources[id]); err != nil {
			errs = append(errs, newErrResource(factoryRemoveMessage, err, id))
		}
	}
	return errors.Join(errs...)
}

// factorySortedIDs returns the sorted keys of m, as a non-nil slice, so
// that an empty open factoryImpl is told apart from a closed one.
func factorySortedIDs[V any](
	m map[string]V,
) []string {
	ids := slices.AppendSeq(make([]string, 0, len(m)), maps.Keys(m))
	slices.Sort(ids)
	return ids
}

// factoryCloseResource closes the resource when it implements io.Closer.
func factoryCloseResource(
	resource any,
) error {
	if closer, ok := resource.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// factoryBagAs returns v as a container.Bag when it is a Bag or a
// map[string]any, sharing the same underlying map, and reports whether it
// was one of those.
func factoryBagAs(
	v any,
) (container.Bag, bool) {
	switch t := v.(type) {
	case container.Bag:
		return t, true
	case map[string]any:
		return t, true
	}
	return nil, false
}
