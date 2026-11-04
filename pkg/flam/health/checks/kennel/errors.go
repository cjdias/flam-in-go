package kennel

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the check configuration, or its default,
	// does not hold a value of the expected type: a kind read by
	// [github.com/cjdias/flam-in-go/pkg/flam/health.ParseKind] and a list
	// of process ids.
	ErrInvalidField = errors.New("invalid kennel health check field")

	// ErrNoKennel is the error wrapped by the error returned from the
	// creator when the container provides no [flam.IKennel], as the
	// bundle of [github.com/cjdias/flam-in-go/pkg/flam/watchdog] is not
	// registered.
	ErrNoKennel = errors.New("no watchdog kennel")

	// ErrUnknownProcess is the error wrapped by the error returned from
	// the creator when the kennel holds no process of an id of its
	// processes field.
	ErrUnknownProcess = errors.New("unknown watchdog process")

	// ErrNotRunning is the error wrapped, along with the ids of the
	// processes, by the error of a check when processes it watches are
	// active in the kennel but not running.
	ErrNotRunning = errors.New("watchdog process not running")
)

// newErr returns the error wrapping the sentinel with the message and the
// ErrorFieldID context field.
func newErr(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}
