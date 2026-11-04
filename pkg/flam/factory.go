package flam

// IResource is the constraint of the resources an [IFactory] holds:
// a resource identifies itself with Id, which returns the id it is created
// for or stored under in the factory. A resource also implementing
// [io.Closer] is closed when removed from the factory.
type IResource interface {
	// Id returns the id of the resource in its factory.
	Id() string
}

// IResourceCreator creates resources of type R from their configuration, for
// an [IFactory]. A factory may hold several creators, for example one per
// database driver, and uses the first one accepting a configuration.
//
// Both methods receive a copy of the resource configuration bag, made for
// that call, which they may freely read or change, and in which the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldId]
// field holds the id of the resource to create: the resource Id method must
// return it. They may be called concurrently.
type IResourceCreator[R IResource] interface {
	// Accept reports whether the creator can create the resource described
	// by the configuration, typically by checking a "driver" or "type"
	// field.
	Accept(bag IBag) bool

	// Create creates the resource described by the configuration. It is
	// only called after Accept returned true for an equal configuration.
	Create(bag IBag) (R, error)
}

// IFactory creates, holds and releases resources of type R, such as
// database connections or clients, described by a configuration and
// identified by string ids.
//
// The configuration given to
// [github.com/cjdias/flam-in-go/pkg/flam/factory.New] is a bag
// holding one resource configuration bag per id. Those ids are the
// available resources: Get creates such a resource on first use and stores
// it, so that later calls return the same one, while Generate creates a new
// one each time without storing it. Resources can also be created elsewhere
// and stored with Store, under any id not in use.
//
// Removing a resource from the factory, with Remove, RemoveAll or Close,
// closes it when it implements [io.Closer]. Once closed, the factory holds
// no resources and every method but Close, Available, Stored, Has and
// Lookup fails with
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ErrClosed].
//
// The errors returned are [*Error] values wrapping one of the sentinel
// errors of the factory package, or the error of a creator or of a
// resource Close, with the "id" context field holding the resource id,
// except for the errors of GenerateAll and RemoveAll on a closed factory,
// which concern no resource. Close and RemoveAll return instead the
// [errors.Join] of such errors, one per failed resource Close, which is not
// itself an *Error, even when a single Close failed.
//
// A creator panic is not recovered: it goes through the Get or Generate
// call that ran the creator, while the concurrent Get calls waiting for
// that resource fail with
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ErrCreatorPanic].
//
// An IFactory is safe for concurrent use. A creator may use the factory
// creating it, for example to get a resource another one depends on, but
// must not Get the very resource it is creating, which would wait for
// itself forever.
type IFactory[R IResource] interface {
	// Close removes every stored resource, closing those implementing
	// io.Closer, and closes the factory. It returns the errors of those
	// Close calls, joined with errors.Join and each wrapped in an *Error
	// holding the resource id, or nil. A resource being created by Get
	// when Close is called is closed once created, and Get fails with
	// factory.ErrClosed. Calling Close again is a no-op returning nil.
	Close() error

	// Available returns, sorted, the ids having a resource configuration,
	// whether or not their resource is stored. It returns nil once the
	// factory is closed.
	Available() []string

	// Stored returns, sorted, the ids of the stored resources, created by
	// Get or GenerateAll or given to Store. It returns nil once the factory
	// is closed.
	Stored() []string

	// Has reports whether Get can return a resource for the id: whether a
	// resource is stored under it, or it has a resource configuration. It
	// returns false once the factory is closed.
	Has(id string) bool

	// Lookup returns the resource stored under the id, and whether there is
	// one, without creating it, unlike Get. It returns false once the
	// factory is closed.
	Lookup(id string) (R, bool)

	// Get returns the resource stored under the id. When there is none
	// yet, it creates it from its configuration, as Generate does, stores
	// it and returns it. Concurrent calls for the same id create the
	// resource once and all return it. Get fails with the errors of
	// Generate, or factory.ErrClosed. When the creator panics, the panic goes
	// through the Get call running it, and the concurrent calls waiting for
	// that resource fail with factory.ErrCreatorPanic.
	Get(id string) (R, error)

	// Store stores a resource created elsewhere under the id, which does
	// not need a configuration. It fails with factory.ErrNilResource when
	// the resource is nil, factory.ErrIDMismatch when its Id is not the id,
	// factory.ErrDuplicateID when a resource is already stored or being
	// created by Get under the id, or factory.ErrClosed. A resource it
	// refuses is left to the caller, not closed.
	Store(id string, resource R) error

	// Generate creates a new resource from the configuration of the id,
	// with the first creator accepting it, without storing it: the caller
	// owns, and must close, the resource. It fails with
	// factory.ErrUnknownID when the id has no configuration,
	// factory.ErrNoCreator when no creator accepts it, the creator error,
	// wrapped, when the creation fails, factory.ErrNilResource when the
	// creator returns a nil resource, factory.ErrIDMismatch when the Id of
	// the created resource is not the id, in which case the resource is
	// closed, or factory.ErrClosed.
	Generate(id string) (R, error)

	// GenerateAll gets, so creates and stores, every available resource not
	// stored yet, in id order. It stops at the first failure, returning
	// the Get error; the resources created before it stay stored.
	GenerateAll() error

	// Remove removes the resource stored under the id, closing it when it
	// implements io.Closer, and returns the Close error, wrapped. The
	// resource is removed even when its Close fails. Remove fails with
	// factory.ErrUnknownID when no resource is stored under the id, or
	// factory.ErrClosed. The configuration of the id is kept, so Get
	// creates a new resource afterwards.
	Remove(id string) error

	// RemoveAll removes every stored resource as Remove does, in id order,
	// and returns their Close errors joined with errors.Join, or nil. It
	// fails with factory.ErrClosed once the factory is closed.
	RemoveAll() error
}
