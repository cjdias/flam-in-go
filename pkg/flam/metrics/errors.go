package metrics

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilCollector is the error wrapped by the error of the container
	// when a collector of the [CollectorGroup] group is nil.
	ErrNilCollector = errors.New("nil metrics collector")

	// ErrRegister is the error wrapped, along with the error of the
	// registry, by the error of the container when a collector of the
	// [CollectorGroup] group cannot be registered, such as when its
	// metrics are those of another collector.
	ErrRegister = errors.New("unable to register metrics collector")
)

// newErrCollector returns the error wrapping the sentinel, joined with the
// cause when not nil, with the message and the ErrorFieldCollector context
// field holding the type of the collector.
func newErrCollector(
	sentinel error,
	message string,
	collector any,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldCollector, fmt.Sprintf("%T", collector))
}
