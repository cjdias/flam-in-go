package health

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilCheck is the error wrapped by the error returned from Add,
	// when the check is nil, and from [NewCheck], when the function is.
	ErrNilCheck = errors.New("nil health check")

	// ErrInvalidID is the error wrapped by the error returned from Add and
	// [NewCheck] for an empty check id.
	ErrInvalidID = errors.New("invalid health check id")

	// ErrInvalidKind is the error wrapped by the error returned from Add
	// and [NewCheck] for a kind other than [flam.HealthLiveness],
	// [flam.HealthReadiness] and [flam.HealthBoth], held by
	// [ErrorFieldKind].
	ErrInvalidKind = errors.New("invalid health check kind")

	// ErrDuplicateCheck is the error wrapped by the error returned from
	// Add when the registry holds a check of the id already.
	ErrDuplicateCheck = errors.New("health check id already in use")

	// ErrInvalidField is the error wrapped by the error of the container
	// when the registry cannot be created, as the value at
	// [TimeoutConfigPath] or [CacheConfigPath] is not a non-negative
	// duration, the path being held by [ErrorFieldField].
	ErrInvalidField = errors.New("invalid health field")

	// ErrCheck is the error wrapped, along with the error of the
	// [flam.IHealthCheckFactory], or of Add, by the error returned from
	// the boot of the bundle when a check cannot be added to the
	// registry.
	ErrCheck = errors.New("unable to add health check")

	// ErrTimeout is the error wrapped by the error reported for a check
	// still running once the timeout of the registry elapsed.
	ErrTimeout = errors.New("health check timed out")

	// ErrPanic is the error wrapped, along with the value it panicked
	// with, by the error reported for a check that panicked.
	ErrPanic = errors.New("health check panicked")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the ErrorFieldID context field.
func newErr(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrPanic returns the error wrapping ErrPanic, joined with the value
// the check of the id panicked with when an error, with the message and
// the ErrorFieldID and ErrorFieldPanic context fields.
func newErrPanic(
	message string,
	id string,
	value any,
) *flam.Error {
	sentinel := fmt.Errorf("%w: %v", ErrPanic, value)
	if err, ok := value.(error); ok {
		sentinel = fmt.Errorf("%w: %w", ErrPanic, err)
	}
	return newErr(sentinel, message, id, nil).
		With(ErrorFieldPanic, value)
}
