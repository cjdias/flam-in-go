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
	// group, a logger string, a list of interceptor ids, certificate and
	// key files both set or both unset, non-negative durations, a positive
	// health interval, positive message sizes and bools.
	ErrInvalidField = errors.New("invalid standard grpc server field")

	// ErrLogger is the error wrapped, along with the error of the
	// [flam.IGrpcLoggerAdaptorFactory], by the error returned from the
	// creator when the logger adaptor cannot be taken from the factory.
	ErrLogger = errors.New("unable to get standard grpc server logger")

	// ErrInterceptor is the error wrapped, along with the error of the
	// [flam.IGrpcInterceptorFactory], by the error returned from the
	// creator when an interceptor cannot be taken from the factory.
	ErrInterceptor = errors.New("unable to get standard grpc server interceptor")

	// ErrCertificate is the error wrapped, along with the error of the
	// loading, by the error returned from the creator when the TLS
	// certificate cannot be loaded from its files.
	ErrCertificate = errors.New("unable to load standard grpc server certificate")

	// ErrRegister is the error wrapped, along with the error of the
	// register, by the error returned from the creator when a service
	// register of the group of the server fails to register its services,
	// or registers an invalid one.
	ErrRegister = errors.New("unable to register standard grpc server services")

	// ErrInvalidService is the error wrapped, along with ErrRegister, by
	// the error returned from the creator when a service is registered
	// twice, such as along with the health or the reflection services of
	// the server, or with an implementation not of its handler type, the
	// name of the service being held by [ErrorFieldService]. The server
	// of gRPC would end the process instead.
	ErrInvalidService = errors.New("invalid standard grpc server service")

	// ErrRunning is the error wrapped by the error returned from Run when
	// the server runs already.
	ErrRunning = errors.New("standard grpc server running")

	// ErrServe is the error wrapped, along with the error of the serving,
	// by the error returned from Run when the server cannot listen on its
	// address, or fails to serve.
	ErrServe = errors.New("unable to serve standard grpc server")

	// ErrShutdown is the error wrapped, along with
	// [context.DeadlineExceeded], by the error returned from Run or Close
	// when the calls under way do not end within the shutdown timeout,
	// their connections being closed.
	ErrShutdown = errors.New("unable to shut down standard grpc server")
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
