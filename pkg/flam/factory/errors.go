package factory

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilCreator is the error wrapped by the error returned from
	// [New] when one of the creators is nil.
	ErrNilCreator = errors.New("nil factory creator")

	// ErrClosed is the error wrapped by the errors returned from the
	// [flam.IFactory] methods, other than Close, once the factory is closed.
	ErrClosed = errors.New("factory closed")

	// ErrUnknownID is the error wrapped by the errors returned from
	// [flam.IFactory] Get and Generate when the id has no resource configuration,
	// and from Remove when no resource is stored under the id.
	ErrUnknownID = errors.New("unknown factory resource id")

	// ErrDuplicateID is the error wrapped by the error returned from
	// [flam.IFactory] Store when a resource is already stored, or being created
	// by Get, under the id.
	ErrDuplicateID = errors.New("factory resource id already in use")

	// ErrNoCreator is the error wrapped by the errors returned from
	// [flam.IFactory] Get and Generate when no creator accepts the resource
	// configuration.
	ErrNoCreator = errors.New("no factory creator accepts the resource")

	// ErrNilResource is the error wrapped by the errors returned from
	// [flam.IFactory] Store, when given a nil resource, and from Get and
	// Generate, when the creator returns a nil resource without an error.
	ErrNilResource = errors.New("nil factory resource")

	// ErrIDMismatch is the error wrapped by the errors returned from
	// [flam.IFactory] Store, when the Id of the resource is not the id it is
	// stored under, and from Get and Generate, when the Id of the created
	// resource is not the id it is created for.
	ErrIDMismatch = errors.New("factory resource id mismatch")

	// ErrCreatorPanic is the error wrapped by the error returned from
	// the [flam.IFactory] Get calls waiting for a resource whose creator
	// panicked. The Get call running the creator panics instead.
	ErrCreatorPanic = errors.New("factory creator panicked")
)

// newErrNilCreator returns the error wrapping ErrNilCreator with the
// message.
func newErrNilCreator(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilCreator, message)
}

// newErrClosed returns the error wrapping ErrClosed with the message.
func newErrClosed(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrClosed, message)
}

// newErrUnknownID returns the error wrapping ErrUnknownID with the message
// and the ErrorFieldID context field.
func newErrUnknownID(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrUnknownID, message).
		With(ErrorFieldID, id)
}

// newErrDuplicateID returns the error wrapping ErrDuplicateID with the
// message and the ErrorFieldID context field.
func newErrDuplicateID(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrDuplicateID, message).
		With(ErrorFieldID, id)
}

// newErrNoCreator returns the error wrapping ErrNoCreator with the message
// and the ErrorFieldID context field.
func newErrNoCreator(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrNoCreator, message).
		With(ErrorFieldID, id)
}

// newErrNilResource returns the error wrapping ErrNilResource with the
// message and the ErrorFieldID context field.
func newErrNilResource(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilResource, message).
		With(ErrorFieldID, id)
}

// newErrIDMismatch returns the error wrapping ErrIDMismatch with the
// message and the ErrorFieldID and ErrorFieldResourceID context fields.
func newErrIDMismatch(
	message string,
	id string,
	resourceID string,
) *flam.Error {
	return flam.NewWrappedError(ErrIDMismatch, message).
		With(ErrorFieldID, id).
		With(ErrorFieldResourceID, resourceID)
}

// newErrCreatorPanic returns the error wrapping ErrCreatorPanic with the
// message and the ErrorFieldID context field.
func newErrCreatorPanic(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrCreatorPanic, message).
		With(ErrorFieldID, id)
}

// newErrResource returns the error wrapping cause with the message and the
// ErrorFieldID context field.
func newErrResource(
	message string,
	cause error,
	id string,
) *flam.Error {
	return flam.NewWrappedError(cause, message).
		With(ErrorFieldID, id)
}
