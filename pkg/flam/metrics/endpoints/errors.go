package endpoints

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrMissingGatherer is the error wrapped by the error returned from
	// the Register method of the register when the container provides no
	// Prometheus gatherer.
	ErrMissingGatherer = errors.New("metrics gatherer not provided by the container")
)

// newErrRegister returns the error wrapping the sentinel with the message.
func newErrRegister(
	sentinel error,
	message string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message)
}
