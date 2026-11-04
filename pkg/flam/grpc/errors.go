package grpc

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrServer is the error wrapped, along with the error of the
	// [flam.IGrpcServerFactory], by the error of the container when a
	// server cannot be created to be added to the kennel.
	ErrServer = errors.New("unable to get grpc server")
)

// newErrJoined returns the error wrapping the sentinel, joined with the
// cause, with the message.
func newErrJoined(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, cause), message)
}
