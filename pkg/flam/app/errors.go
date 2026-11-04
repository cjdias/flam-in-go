package app

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilBundle is the error wrapped by the error returned
	// from [flam.IApp.RegisterBundle] when the bundle is nil.
	ErrNilBundle = errors.New("nil application bundle")

	// ErrDuplicateBundle is the error wrapped by the error
	// returned from [flam.IApp.RegisterBundle] when a bundle with the
	// same id is already registered.
	ErrDuplicateBundle = errors.New("application bundle id already registered")

	// ErrBooted is the error wrapped by the error returned from
	// [flam.IApp.RegisterBundle] once the application is booted.
	ErrBooted = errors.New("application already booted")

	// ErrInvalidFunction is the error wrapped by the errors
	// returned from [flam.IApp] Provide, Invoke and Decorate when given
	// something else than a non-nil function. The dig.Container methods do
	// not all refuse those: Decorate panics on a value that is not a
	// function, Invoke panics on a nil function, and Provide and Decorate
	// accept a nil function, which fails only once used.
	ErrInvalidFunction = errors.New("invalid application function")

	// ErrBooting is the error wrapped by the error returned from
	// [flam.IApp.Boot] and [flam.IApp.Run] when called back by a bundle
	// while the application boots, as the boot is not over, and the
	// bundles not booted yet cannot run.
	ErrBooting = errors.New("application booting")

	// ErrRunning is the error wrapped by the error returned from
	// [flam.IApp.Run] when called back by a bundle while the application
	// runs, which would run the bundles again, without end.
	ErrRunning = errors.New("application running")

	// ErrClosed is the error wrapped by the errors returned from
	// the [flam.IApp] methods, other than Close and the read only ones,
	// once the application is closed.
	ErrClosed = errors.New("application closed")
)

// newErrNilBundle returns the error wrapping ErrNilBundle with the
// message.
func newErrNilBundle(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilBundle, message)
}

// newErrDuplicateBundle returns the error wrapping ErrDuplicateBundle with
// the message and the ErrorFieldBundle context field.
func newErrDuplicateBundle(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrDuplicateBundle, message).
		With(ErrorFieldBundle, id)
}

// newErrBooted returns the error wrapping ErrBooted with the message.
func newErrBooted(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrBooted, message)
}

// newErrInvalidFunction returns the error wrapping ErrInvalidFunction with
// the message.
func newErrInvalidFunction(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidFunction, message)
}

// newErrClosed returns the error wrapping ErrClosed with the message.
func newErrClosed(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrClosed, message)
}

// newErrBooting returns the error wrapping ErrBooting with the message.
func newErrBooting(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrBooting, message)
}

// newErrRunning returns the error wrapping ErrRunning with the message.
func newErrRunning(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrRunning, message)
}

// newErrWrapped returns the error wrapping err with the message: the error
// of the container, or of the function it called, or the registration
// failures failing the boot.
func newErrWrapped(
	message string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message)
}

// newErrBundle returns the error wrapping the error of a bundle, or of the
// configuration refusing its defaults, with the message and the
// ErrorFieldBundle context field.
func newErrBundle(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldBundle, id)
}
