package leader

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilLocker is the error wrapped by the error returned from
	// [NewProcess] when the locker is nil.
	ErrNilLocker = errors.New("nil leader locker")

	// ErrNilRun is the error wrapped by the error returned from
	// [NewProcess] when the function is nil.
	ErrNilRun = errors.New("nil leader function")

	// ErrInvalidKey is the error wrapped by the error returned from
	// [NewProcess] when the key is empty.
	ErrInvalidKey = errors.New("invalid leader key")

	// ErrInvalidTTL is the error wrapped by the error returned from
	// [NewProcess] when the ttl is shorter than 1ms.
	ErrInvalidTTL = errors.New("invalid leader ttl")

	// ErrRunning is the error wrapped by the error returned from the Run
	// method of a process of [NewProcess] when it runs already.
	ErrRunning = errors.New("leader process running")
)

// newErr returns the error wrapping the sentinel with the message and the
// ErrorFieldID and ErrorFieldKey context fields.
func newErr(
	sentinel error,
	message string,
	id string,
	key string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldKey, key)
}
