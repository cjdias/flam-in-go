package mongo

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilClient is the error wrapped by the error returned from
	// [NewConnection] when the client is nil.
	ErrNilClient = errors.New("nil mongo client")

	// ErrEmptyDatabase is the error wrapped by the error returned from
	// [NewConnection] when the name of the default database is empty.
	ErrEmptyDatabase = errors.New("empty mongo database name")
)

// newErrNew returns the error wrapping the sentinel of an invalid argument
// of NewConnection with the ErrorFieldID context field.
func newErrNew(
	sentinel error,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, connectionNewMessage).
		With(ErrorFieldID, id)
}

// newErrConnection returns the error wrapping the error of the client with
// the message and the ErrorFieldID context field.
func newErrConnection(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}
