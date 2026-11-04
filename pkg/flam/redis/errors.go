package redis

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrNilClient is the error wrapped by the error returned from
// [NewConnection] when the client is nil.
var ErrNilClient = errors.New("nil redis client")

// newErrNil returns the error wrapping the sentinel of a nil argument with
// the message and the ErrorFieldID context field.
func newErrNil(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}
