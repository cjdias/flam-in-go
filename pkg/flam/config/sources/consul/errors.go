package consul

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the source configuration, or its default,
	// does not hold a valid value: an int priority, a bool reload, an
	// absolute http or https address, a non-empty key not starting with a
	// slash, a bool recursive, strings for the token, the datacenter, the
	// parser and the files, a parser unless recursive, a certificate file
	// and a key file set together, a duration, or duration string, timeout
	// of zero or more, and a bool skip_verify.
	ErrInvalidField = errors.New("invalid consul config source field")

	// ErrParser is the error wrapped, along with the error of the
	// [flam.IConfigParserFactory], or ErrMissingFactory, by the error
	// returned from the creator when the parser cannot be taken from the
	// factory.
	ErrParser = errors.New("unable to get consul config source parser")

	// ErrMissingFactory is the error wrapped, along with ErrParser, when
	// the container provides no [flam.IConfigParserFactory].
	ErrMissingFactory = errors.New("factory not provided by the container")

	// ErrClient is the error wrapped, along with the error of the Consul
	// client, by the error returned from the creator when the client
	// cannot be created, such as for a certificate file that cannot be
	// loaded.
	ErrClient = errors.New("unable to create consul config source client")

	// ErrRequest is the error wrapped, along with the error of the Consul
	// client, by the error returned from the creator or Reload when a
	// request fails, such as for an agent that cannot be reached, a status
	// other than 200 and 404, or a timeout.
	ErrRequest = errors.New("consul config request failed")

	// ErrMissingKey is the error wrapped by the error returned from the
	// creator or Reload when the key of a source that is not recursive does
	// not exist.
	ErrMissingKey = errors.New("consul config key not found")

	// ErrParse is the error wrapped, along with the error of the parser, by
	// the error returned from the creator or Reload when a value cannot be
	// parsed.
	ErrParse = errors.New("unable to parse consul config value")

	// ErrTree is the error wrapped, along with the error of the bag, by the
	// error returned from the creator or Reload when a key of the tree of a
	// recursive source cannot be stored at its path, such as for a key
	// holding an empty element, or one under a key holding a string.
	ErrTree = errors.New("unable to store consul config key")
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

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return newErr(ErrInvalidField, message, id, nil).
		With(ErrorFieldField, field)
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
	return newErr(ErrParser, message, id, err).
		With(ErrorFieldParser, parser)
}

// newErrKey returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the ErrorFieldID and ErrorFieldKey
// context fields.
func newErrKey(
	sentinel error,
	message string,
	id string,
	key string,
	cause error,
) *flam.Error {
	return newErr(sentinel, message, id, cause).
		With(ErrorFieldKey, key)
}
