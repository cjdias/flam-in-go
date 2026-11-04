package env

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the source configuration, or its default,
	// does not hold a value of the expected type: an int priority, a list
	// of strings for the files, or a bag for the mappings.
	ErrInvalidField = errors.New("invalid env config source field")

	// ErrInvalidMapping is the error wrapped by the error returned from the
	// creator when a mapping has an empty variable name, or a path that is
	// not a string or not a valid path, such as "" or "a..b".
	ErrInvalidMapping = errors.New("invalid env config source mapping")

	// ErrMappingConflict is the error wrapped by the error returned from the
	// creator when two mappings write over each other: they have the same
	// path, or the path of one is inside the path of the other.
	ErrMappingConflict = errors.New("conflicting env config source mappings")

	// ErrRead is the error wrapped, along with the error of the dotenv
	// reader, by the error returned from the creator or Reload when a
	// dotenv file cannot be read or parsed.
	ErrRead = errors.New("unable to read env config files")
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

// newErrInvalidMapping returns the error wrapping ErrInvalidMapping with
// the message and the ErrorFieldID, ErrorFieldKey and ErrorFieldPath
// context fields.
func newErrInvalidMapping(
	message string,
	id string,
	key string,
	path any,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidMapping, message).
		With(ErrorFieldID, id).
		With(ErrorFieldKey, key).
		With(ErrorFieldPath, path)
}

// newErrMappingConflict returns the error wrapping ErrMappingConflict with
// the message and the ErrorFieldID, ErrorFieldKey, ErrorFieldPath,
// ErrorFieldOtherKey and ErrorFieldOtherPath context fields.
func newErrMappingConflict(
	message string,
	id string,
	key string,
	path string,
	otherKey string,
	otherPath string,
) *flam.Error {
	return flam.NewWrappedError(ErrMappingConflict, message).
		With(ErrorFieldID, id).
		With(ErrorFieldKey, key).
		With(ErrorFieldPath, path).
		With(ErrorFieldOtherKey, otherKey).
		With(ErrorFieldOtherPath, otherPath)
}

// newErrRead returns the error wrapping ErrRead, joined with the error of
// the dotenv reader, with the message and the ErrorFieldID and
// ErrorFieldFiles context fields.
func newErrRead(
	message string,
	id string,
	files []string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRead, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldFiles, files)
}
