package standard

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the server configuration, or its default,
	// does not hold a value of the expected type: a non-empty address and
	// group, a logger string, a list of middleware ids, certificate and
	// key files both set or both unset, non-negative durations and a
	// positive maximum header size, or when the service id at
	// [github.com/cjdias/flam-in-go/pkg/flam/rest.ServiceIdConfigPath] is
	// not a whole number, the field then being that path.
	ErrInvalidField = errors.New("invalid standard rest server field")

	// ErrLogger is the error wrapped, along with the error of the
	// [flam.IRestLoggerAdaptorFactory], by the error returned from the
	// creator when the logger adaptor cannot be taken from the factory.
	ErrLogger = errors.New("unable to get standard rest server logger")

	// ErrMiddleware is the error wrapped, along with the error of the
	// [flam.IRestMiddlewareFactory], by the error returned from the
	// creator when a middleware cannot be taken from the factory.
	ErrMiddleware = errors.New("unable to get standard rest server middleware")

	// ErrCertificate is the error wrapped, along with the error of the
	// loading, by the error returned from the creator when the TLS
	// certificate cannot be loaded from its files.
	ErrCertificate = errors.New("unable to load standard rest server certificate")

	// ErrRegister is the error wrapped, along with the error of the
	// register, by the error returned from the creator when an endpoint
	// register of the group of the server fails to register its
	// endpoints.
	ErrRegister = errors.New("unable to register standard rest server endpoints")

	// ErrRunning is the error wrapped by the error returned from Run when
	// the server runs already.
	ErrRunning = errors.New("standard rest server running")

	// ErrServe is the error wrapped, along with the error of the serving,
	// by the error returned from Run when the server cannot listen on its
	// address, or fails to serve.
	ErrServe = errors.New("unable to serve standard rest server")

	// ErrShutdown is the error wrapped, along with the error of the
	// shutdown, by the error returned from Run or Close when the requests
	// under way do not end within the shutdown timeout, their connections
	// being closed.
	ErrShutdown = errors.New("unable to shut down standard rest server")
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
