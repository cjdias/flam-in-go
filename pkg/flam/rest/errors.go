package rest

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilMiddleware is the error wrapped by the error returned from
	// [NewRouter] when one of its middlewares is nil.
	ErrNilMiddleware = errors.New("nil rest middleware")

	// ErrInvalidEndpoint is the error wrapped by the error returned from
	// the Handle method of a router when the endpoint has no pattern, an
	// invalid pattern, or a pattern conflicting with the pattern of
	// another endpoint, a status out of the 200 to 599 range, other than
	// 0, or a nil handler, the field being held by [ErrorFieldField].
	ErrInvalidEndpoint = errors.New("invalid rest endpoint")

	// ErrMiddleware is the error wrapped by the error returned from
	// [NewRouter], and from the Handle method of a router, when a
	// middleware wraps the handler of the requests matching no pattern, or
	// of the endpoint, into a nil handler.
	ErrMiddleware = errors.New("rest middleware returned no handler")

	// ErrNotFound is the error of the envelope answering, with a 404
	// status, a request whose path matches the pattern of no endpoint of a
	// [Router].
	ErrNotFound = errors.New("rest route not found")

	// ErrMethodNotAllowed is the error of the envelope answering, with a
	// 405 status, a request whose path matches the pattern of an endpoint
	// of a [Router], but not its method.
	ErrMethodNotAllowed = errors.New("rest method not allowed")

	// ErrServer is the error wrapped, along with the error of the
	// [flam.IRestServerFactory], by the error of the container when a
	// server cannot be created to be added to the kennel.
	ErrServer = errors.New("unable to get rest server")

	// ErrDecode is the error wrapped, along with the error of the decoder,
	// by the errors returned from [DecodePath], [DecodeQuery] and
	// [DecodeJSON], the part of the request being held by
	// [ErrorFieldSource].
	ErrDecode = errors.New("invalid rest request")

	// ErrEncode is the error wrapped, along with the error of the encoder,
	// by the error returned from [WriteEnvelope] when the envelope cannot
	// be encoded, in which case nothing is written.
	ErrEncode = errors.New("unable to encode rest response")

	// ErrWrite is the error wrapped, along with the error of the response
	// writer, by the error returned from [WriteEnvelope] when the response
	// cannot be written.
	ErrWrite = errors.New("unable to write rest response")
)

// newErrEndpoint returns the error wrapping the sentinel, joined with the
// cause when not nil, with the message and the ErrorFieldEndpoint and
// ErrorFieldPattern context fields of the endpoint.
func newErrEndpoint(
	sentinel error,
	message string,
	endpoint flam.RestEndpoint,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldEndpoint, endpoint.Id).
		With(ErrorFieldPattern, endpoint.Pattern)
}

// newErrJoined returns the error wrapping the sentinel, joined with the
// cause, with the message.
func newErrJoined(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, cause), message)
}
