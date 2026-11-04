package database

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrPlugin is the error wrapped, along with the error of gorm, by the
// error returned from a connection creator decorated by the bundle
// returned from [NewBundle] when the plugin cannot be added to the
// connection it created, such as when the callbacks of the connection
// cannot be ordered. The connection is then closed.
var ErrPlugin = errors.New("unable to add tracing plugin to database connection")

// newErrPlugin returns the error wrapping ErrPlugin, joined with the
// cause, with the message and the ErrorFieldID context field.
func newErrPlugin(
	message string,
	id string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrPlugin, cause), message).
		With(ErrorFieldID, id)
}
