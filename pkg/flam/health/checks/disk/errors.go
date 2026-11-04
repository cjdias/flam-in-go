package disk

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the check configuration, or its default,
	// does not hold a value of the expected type: a kind read by
	// [github.com/cjdias/flam-in-go/pkg/flam/health.ParseKind], a
	// non-empty disk id and a non-empty path.
	ErrInvalidField = errors.New("invalid disk health check field")

	// ErrNoFactory is the error wrapped by the error returned from the
	// creator when the container provides no [flam.IDiskFactory], as the
	// bundle of [github.com/cjdias/flam-in-go/pkg/flam/disk] is not
	// registered.
	ErrNoFactory = errors.New("no disk factory")

	// ErrUnknownDisk is the error wrapped by the error returned from the
	// creator when the disk factory has no disk of the id.
	ErrUnknownDisk = errors.New("unknown disk")

	// ErrDisk is the error wrapped, along with the error of the disk
	// factory, by the error of a check when the disk cannot be taken.
	ErrDisk = errors.New("unable to get disk")

	// ErrStat is the error wrapped, along with the error of the disk, by
	// the error of a check when the path cannot be stated, such as when
	// it does not exist or the backend does not answer.
	ErrStat = errors.New("unable to stat disk path")
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
