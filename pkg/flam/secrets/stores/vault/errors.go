package vault

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the store configuration, or its default,
	// does not hold a value of the expected type: an absolute http or https
	// address, a non-empty mount of valid elements, strings for the
	// namespace, the token and the files, non-negative durations and a
	// skip_verify bool, or when no token is configured nor set in
	// [EnvToken], the field then being [ConfigFieldToken].
	ErrInvalidField = errors.New("invalid vault secret store field")

	// ErrCAFile is the error wrapped, along with the error of the reading
	// when there is one, by the error returned from the creator when the
	// file of [ConfigFieldCAFile] cannot be read, or holds no PEM
	// certificate.
	ErrCAFile = errors.New("unable to load vault certificate authorities")

	// ErrToken is the error wrapped, along with the error of the reading
	// when there is one, by the error returned from the stores when the
	// file of [ConfigFieldTokenFile] cannot be read, or is empty.
	ErrToken = errors.New("unable to read vault token file")

	// ErrRequest is the error wrapped, along with the error of the HTTP
	// client, by the error returned from the stores when a request fails,
	// such as for a server that cannot be reached, or a timeout.
	ErrRequest = errors.New("unable to send vault request")

	// ErrStatus is the error wrapped by the error returned from the stores
	// when a response has a status other than 200 and 404, such as 403 for
	// a token that is not allowed to read the secret.
	ErrStatus = errors.New("unexpected vault response status")

	// ErrResponse is the error wrapped, along with the error of the
	// decoding, by the error returned from the stores when a response body
	// is not the JSON of a KV version 2 secret.
	ErrResponse = errors.New("invalid vault response")
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
