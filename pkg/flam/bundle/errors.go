package bundle

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilApplication is the error wrapped by the error
	// returned from [flam.IRegisterer] Register when the registerer was
	// built for a nil application, including a nil pointer held by a
	// non-nil IApp.
	ErrNilApplication = errors.New("nil registerer application")

	// ErrNilBundle is the error wrapped by the error returned
	// from [flam.IRegisterer] Register when a queued bundle is nil.
	ErrNilBundle = errors.New("nil registerer bundle")

	// ErrInvalidFunction is the error wrapped by the error
	// returned from [flam.IRegisterer] Register when a queued provider,
	// invoker or decorator is not a non-nil function.
	ErrInvalidFunction = errors.New("invalid registerer function")

	// ErrRegistering is the error wrapped by the error returned
	// from [flam.IRegisterer] Register when called while another Register
	// call of the same registerer is applying entries, such as from an
	// invoker or a bundle it applies.
	ErrRegistering = errors.New("registerer already registering")
)

// newErrNilApplication returns the error wrapping ErrNilApplication with
// the message.
func newErrNilApplication(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilApplication, message)
}

// newErrRegistering returns the error wrapping ErrRegistering with the
// message.
func newErrRegistering(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrRegistering, message)
}

// newErrNilBundle returns the error wrapping ErrNilBundle with the message
// and the ErrorFieldIndex context field, and the ErrorFieldKind one set to
// the bundle kind.
func newErrNilBundle(
	message string,
	index int,
) *flam.Error {
	return flam.NewWrappedError(ErrNilBundle, message).
		With(ErrorFieldIndex, index).
		With(ErrorFieldKind, registererKindBundle)
}

// newErrInvalidFunction returns the error wrapping ErrInvalidFunction with
// the message and the ErrorFieldIndex and ErrorFieldKind context fields.
func newErrInvalidFunction(
	message string,
	index int,
	kind string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidFunction, message).
		With(ErrorFieldIndex, index).
		With(ErrorFieldKind, kind)
}

// newErrApplication returns the error wrapping cause with the message and
// the ErrorFieldIndex and ErrorFieldKind context fields.
func newErrApplication(
	message string,
	cause error,
	index int,
	kind string,
) *flam.Error {
	return flam.NewWrappedError(cause, message).
		With(ErrorFieldIndex, index).
		With(ErrorFieldKind, kind)
}
