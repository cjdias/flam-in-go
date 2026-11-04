package rest

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the source configuration, or its default,
	// does not hold a valid value: an int priority, a bool reload, an
	// absolute URI, a valid HTTP method, a bag of string headers, a
	// duration, or duration string, timeout of zero or more, a positive int
	// maximum size, a string parser and valid, or empty, string paths.
	ErrInvalidField = errors.New("invalid rest config source field")

	// ErrClient is the error wrapped, along with the error of the
	// [flam.IConfigRestClientCreator], or ErrNilClient, by the error
	// returned from the creator when the client cannot be created.
	ErrClient = errors.New("unable to create rest config source client")

	// ErrNilClient is the error wrapped, along with ErrClient, when the
	// client creator returns a nil client.
	ErrNilClient = errors.New("nil rest config client")

	// ErrParser is the error wrapped, along with the error of the
	// [flam.IConfigParserFactory], or ErrMissingFactory, by the error
	// returned from the creator or Reload when the parser cannot be taken
	// from the factory.
	ErrParser = errors.New("unable to get rest config source parser")

	// ErrMissingFactory is the error wrapped, along with ErrParser, when
	// the container provides no [flam.IConfigParserFactory].
	ErrMissingFactory = errors.New("factory not provided by the container")

	// ErrRequest is the error wrapped, along with the error of the client,
	// by the error returned from the creator or Reload when the request
	// fails.
	ErrRequest = errors.New("rest config request failed")

	// ErrNoResponse is the error wrapped, along with ErrRequest, when the
	// client returns neither a response nor an error.
	ErrNoResponse = errors.New("no rest config response")

	// ErrStatus is the error wrapped by the error returned from the
	// creator or Reload when the status of the response is not 2xx.
	ErrStatus = errors.New("unexpected rest config response status")

	// ErrContentType is the error wrapped by the error returned from the
	// creator or Reload when no parser is configured and the media type of
	// the response names none.
	ErrContentType = errors.New("unsupported rest config response content type")

	// ErrParse is the error wrapped, along with the error of the parser, by
	// the error returned from the creator or Reload when the response body
	// cannot be read or parsed.
	ErrParse = errors.New("unable to parse rest config response")

	// ErrTooLarge is the error wrapped by the error returned from the
	// creator or Reload when the response body is larger than the maximum
	// size of the source.
	ErrTooLarge = errors.New("rest config response too large")

	// ErrResponsePath is the error wrapped by the error returned from the
	// creator or Reload when the decoded response holds no bag at the
	// configured path.
	ErrResponsePath = errors.New("no config bag at the rest response path")

	// ErrTimestamp is the error wrapped by the error returned from the
	// creator or Reload when a timestamp path is configured and the decoded
	// response holds no string or number at it.
	ErrTimestamp = errors.New("no timestamp at the rest response timestamp path")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrClient returns the error wrapping ErrClient, joined with the cause,
// with the message and the ErrorFieldID context field.
func newErrClient(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrClient, err), message).
		With(ErrorFieldID, id)
}

// newErrParser returns the error wrapping ErrParser, joined with the cause,
// with the message and the ErrorFieldID and ErrorFieldParser context
// fields.
func newErrParser(
	message string,
	id string,
	parser string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrParser, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldParser, parser)
}

// newErrRequest returns the error wrapping ErrRequest, joined with the
// error of the client, with the message and the ErrorFieldID,
// ErrorFieldMethod and ErrorFieldURI context fields.
func newErrRequest(
	message string,
	id string,
	method string,
	uri string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRequest, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldMethod, method).
		With(ErrorFieldURI, uri)
}

// newErrStatus returns the error wrapping ErrStatus with the message and
// the ErrorFieldID, ErrorFieldMethod, ErrorFieldURI and ErrorFieldStatus
// context fields.
func newErrStatus(
	message string,
	id string,
	method string,
	uri string,
	status int,
) *flam.Error {
	return flam.NewWrappedError(ErrStatus, message).
		With(ErrorFieldID, id).
		With(ErrorFieldMethod, method).
		With(ErrorFieldURI, uri).
		With(ErrorFieldStatus, status)
}

// newErrContentType returns the error wrapping ErrContentType with the
// message and the ErrorFieldID, ErrorFieldURI and ErrorFieldContentType
// context fields.
func newErrContentType(
	message string,
	id string,
	uri string,
	contentType string,
) *flam.Error {
	return flam.NewWrappedError(ErrContentType, message).
		With(ErrorFieldID, id).
		With(ErrorFieldURI, uri).
		With(ErrorFieldContentType, contentType)
}

// newErrParse returns the error wrapping ErrParse, joined with the error of
// the parser, with the message and the ErrorFieldID, ErrorFieldURI and
// ErrorFieldParser context fields.
func newErrParse(
	message string,
	id string,
	uri string,
	parser string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrParse, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldURI, uri).
		With(ErrorFieldParser, parser)
}

// newErrResponsePath returns the error wrapping ErrResponsePath with the
// message and the ErrorFieldID, ErrorFieldURI and ErrorFieldPath context
// fields.
func newErrResponsePath(
	message string,
	id string,
	uri string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrResponsePath, message).
		With(ErrorFieldID, id).
		With(ErrorFieldURI, uri).
		With(ErrorFieldPath, path)
}

// newErrTimestamp returns the error wrapping ErrTimestamp with the message
// and the ErrorFieldID, ErrorFieldURI and ErrorFieldTimestampPath context
// fields.
func newErrTimestamp(
	message string,
	id string,
	uri string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrTimestamp, message).
		With(ErrorFieldID, id).
		With(ErrorFieldURI, uri).
		With(ErrorFieldTimestampPath, path)
}

// newErrTooLarge returns the error wrapping ErrTooLarge with the message
// and the ErrorFieldID, ErrorFieldURI and ErrorFieldMaxSize context fields.
func newErrTooLarge(
	message string,
	id string,
	uri string,
	maxSize int,
) *flam.Error {
	return flam.NewWrappedError(ErrTooLarge, message).
		With(ErrorFieldID, id).
		With(ErrorFieldURI, uri).
		With(ErrorFieldMaxSize, maxSize)
}
