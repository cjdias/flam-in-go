package yaml

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrRead is the error wrapped, along with the error of the reader, by
	// the error returned from Parse when the reader fails.
	ErrRead = errors.New("unable to read yaml config document")

	// ErrInvalidDocument is the error wrapped, along with the error of the
	// YAML library, by the error returned from Parse when the content is
	// not valid YAML, holds a key twice in a mapping or more than one
	// document.
	ErrInvalidDocument = errors.New("invalid yaml config document")

	// ErrNotMapping is the error wrapped by the error returned from Parse
	// when the document is valid YAML but not a mapping, such as a
	// sequence.
	ErrNotMapping = errors.New("yaml config document is not a mapping")
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
// joined with the error of the YAML library, with the message and the
// ErrorFieldID context field.
func newErrInvalidDocument(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrInvalidDocument, err), message).
		With(ErrorFieldID, id)
}

// newErrNotMapping returns the error wrapping ErrNotMapping with the
// message and the ErrorFieldID context field.
func newErrNotMapping(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrNotMapping, message).
		With(ErrorFieldID, id)
}
