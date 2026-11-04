package file

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the source configuration, or its default,
	// does not hold a value of the expected type: an int priority and
	// string disk, path and parser. It is also returned for a missing or
	// empty path, and for a missing parser when the file has no extension
	// to name it after.
	ErrInvalidField = errors.New("invalid file config source field")

	// ErrDisk is the error wrapped, along with the error of the
	// [flam.IDiskFactory], by the error returned from the creator when the
	// disk cannot be taken from the factory.
	ErrDisk = errors.New("unable to get file config source disk")

	// ErrParser is the error wrapped, along with the error of the
	// [flam.IConfigParserFactory], by the error returned from the creator
	// when the parser cannot be taken from the factory.
	ErrParser = errors.New("unable to get file config source parser")

	// ErrMissingFactory is the error wrapped, along with ErrDisk or
	// ErrParser, by the error returned from the creator when the container
	// provides no [flam.IDiskFactory] or no [flam.IConfigParserFactory].
	ErrMissingFactory = errors.New("factory not provided by the container")

	// ErrRead is the error wrapped, along with the error of the disk, by
	// the error returned from the creator or Reload when the file cannot be
	// opened.
	ErrRead = errors.New("unable to read config file")

	// ErrParse is the error wrapped, along with the error of the parser, by
	// the error returned from the creator or Reload when the file cannot be
	// read or parsed by the parser.
	ErrParse = errors.New("unable to parse config file")
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

// newErrDisk returns the error wrapping ErrDisk, joined with the error of
// the disk factory, with the message and the ErrorFieldID and
// ErrorFieldDisk context fields.
func newErrDisk(
	message string,
	id string,
	disk string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrDisk, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldDisk, disk)
}

// newErrParser returns the error wrapping ErrParser, joined with the error
// of the parser factory, with the message and the ErrorFieldID and
// ErrorFieldParser context fields.
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

// newErrRead returns the error wrapping ErrRead, joined with the error of
// the disk, with the message and the ErrorFieldID, ErrorFieldDisk and
// ErrorFieldPath context fields.
func newErrRead(
	message string,
	id string,
	disk string,
	path string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRead, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldDisk, disk).
		With(ErrorFieldPath, path)
}

// newErrParse returns the error wrapping ErrParse, joined with the error of
// the parser, with the message and the ErrorFieldID, ErrorFieldPath and
// ErrorFieldParser context fields.
func newErrParse(
	message string,
	id string,
	path string,
	parser string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrParse, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldPath, path).
		With(ErrorFieldParser, parser)
}
