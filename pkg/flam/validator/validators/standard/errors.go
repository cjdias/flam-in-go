package standard

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when the parser field of the validator configuration, or its
	// default, does not hold a non-empty id, or the converter field a
	// string.
	ErrInvalidField = errors.New("invalid standard validator field")

	// ErrParser is the error wrapped, along with the error of the
	// [flam.IValidationParserFactory], by the error returned from the
	// creator when the parser cannot be taken from the factory.
	ErrParser = errors.New("unable to get standard validator parser")

	// ErrConverter is the error wrapped, along with the error of the
	// [flam.IValidationErrorConverterFactory], by the error returned from
	// the creator when the converter cannot be taken from the factory.
	ErrConverter = errors.New("unable to get standard validator converter")

	// ErrNilParser is the error wrapped by the error returned from
	// [NewValidator] when the parser is nil.
	ErrNilParser = errors.New("nil standard validator parser")

	// ErrRegister is the error wrapped, along with the error of the
	// parser, by the error returned from [NewValidator] when the parser
	// fails to register its messages.
	ErrRegister = errors.New("unable to register standard validator parser")
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

// newErrJoined returns the error wrapping the sentinel, joined with the
// error of a factory or the parser, with the message and the ErrorFieldID
// context field, and the field of the id taken from the factory, if any.
func newErrJoined(
	sentinel error,
	message string,
	id string,
	field string,
	value string,
	err error,
) *flam.Error {
	e := flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, err), message).
		With(ErrorFieldID, id)
	if field != "" {
		e = e.With(field, value)
	}
	return e
}

// newErrNil returns the error wrapping the sentinel of a nil argument with
// the message and the ErrorFieldID context field.
func newErrNil(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrInvalidValue returns the error wrapping validator.ErrInvalidValue,
// joined with the error of go-playground, with the message and the
// validator.ErrorFieldID and validator.ErrorFieldType context fields.
func newErrInvalidValue(
	message string,
	id string,
	value any,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", validator.ErrInvalidValue, err), message).
		With(validator.ErrorFieldID, id).
		With(validator.ErrorFieldType, fmt.Sprintf("%T", value))
}
