package middleware

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a namespace
	// string, a non-empty list of increasing buckets and a bool
	// code_class.
	ErrInvalidField = errors.New("invalid metrics rest middleware field")

	// ErrMissingRegisterer is the error wrapped by the error returned from
	// the creator when the container provides no Prometheus registerer.
	ErrMissingRegisterer = errors.New("metrics registerer not provided by the container")

	// ErrRegister is the error wrapped, along with the error of the
	// registerer, by the error returned from the creator when the metrics
	// cannot be registered, such as when their namespace is not a valid
	// metric name, or when other metrics of the same names are
	// registered.
	ErrRegister = errors.New("unable to register metrics rest middleware metrics")
)

// newErrCreate returns the error wrapping the sentinel, joined with the
// cause when not nil, with the message and the ErrorFieldID context field,
// and the ErrorFieldField one when field is not empty.
func newErrCreate(
	sentinel error,
	message string,
	id string,
	field string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	e := flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
	if field != "" {
		e = e.With(ErrorFieldField, field)
	}
	return e
}
