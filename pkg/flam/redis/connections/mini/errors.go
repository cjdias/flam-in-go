package mini

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the connection configuration, or its
	// default, does not hold a value of the expected type and range.
	ErrInvalidField = errors.New("invalid mini redis connection field")

	// ErrStart is the error wrapped, along with the error of the server,
	// by the error returned from the creator when the server of the
	// connection cannot listen on its address.
	ErrStart = errors.New("unable to start mini redis server")
)

// newErrStart returns the error wrapping ErrStart, joined with the error
// of the server, with the message and the ErrorFieldID and
// ErrorFieldAddress context fields.
func newErrStart(
	message string,
	id string,
	address string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrStart, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldAddress, address)
}
