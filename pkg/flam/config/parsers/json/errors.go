package json

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrRead is the error wrapped, along with the error of the reader, by
	// the error returned from Parse when the reader fails.
	ErrRead = errors.New("unable to read json config document")

	// ErrInvalidDocument is the error wrapped, along with the error of
	// encoding/json, by the error returned from Parse when the content is
	// not valid JSON, or is followed by more data.
	ErrInvalidDocument = errors.New("invalid json config document")

	// ErrNotObject is the error wrapped by the error returned from Parse
	// when the document is valid JSON but not an object, such as an array.
	ErrNotObject = errors.New("json config document is not an object")
)

// newErrRead returns the error wrapping ErrRead, joined with the error of
// the reader, with the message and the ErrorFieldID context field.
func newErrRead(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRead, err), message).
		With(ErrorFieldID, id)
}

// newErrInvalidDocument returns the error wrapping ErrInvalidDocument,
// joined with the error of encoding/json, with the message and the
// ErrorFieldID context field.
func newErrInvalidDocument(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrInvalidDocument, err), message).
		With(ErrorFieldID, id)
}

// newErrNotObject returns the error wrapping ErrNotObject with the message
// and the ErrorFieldID context field.
func newErrNotObject(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrNotObject, message).
		With(ErrorFieldID, id)
}
