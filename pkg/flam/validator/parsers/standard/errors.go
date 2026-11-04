package standard

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the parser configuration, or its default,
	// does not hold a string.
	ErrInvalidField = errors.New("invalid standard validation parser field")

	// ErrTranslator is the error wrapped, along with the error of the
	// [flam.ITranslatorFactory], by the error returned from the creator
	// when the translator cannot be taken from the factory, or when no
	// bundle provides the factory.
	ErrTranslator = errors.New("unable to get standard validation parser translator")

	// ErrLocale is the error wrapped by the error returned from
	// [NewParser] when the locale of the translator has no messages.
	ErrLocale = errors.New("unsupported standard validation parser locale")

	// ErrNilValidate is the error wrapped by the error returned from the
	// Register method of a parser when the validate is nil.
	ErrNilValidate = errors.New("nil standard validation parser validate")

	// ErrRegister is the error wrapped, along with the error of
	// go-playground, by the error returned from the Register method of a
	// parser when the messages cannot be registered.
	ErrRegister = errors.New("unable to register standard validation parser messages")
)

// errNoTranslators is the cause of the ErrTranslator errors when no bundle
// provides the translator factory.
var errNoTranslators = errors.New("no translator factory")

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

// newErrTranslator returns the error wrapping ErrTranslator, joined with
// the cause, with the message and the ErrorFieldID and
// ErrorFieldTranslator context fields.
func newErrTranslator(
	message string,
	id string,
	translator string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrTranslator, cause), message).
		With(ErrorFieldID, id).
		With(ErrorFieldTranslator, translator)
}

// newErrLocale returns the error wrapping ErrLocale with the message and
// the ErrorFieldID and ErrorFieldLocale context fields.
func newErrLocale(
	message string,
	id string,
	locale string,
) *flam.Error {
	return flam.NewWrappedError(ErrLocale, message).
		With(ErrorFieldID, id).
		With(ErrorFieldLocale, locale)
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

// newErrRegister returns the error wrapping ErrRegister, joined with the
// error of go-playground, with the message and the ErrorFieldID and
// ErrorFieldLocale context fields.
func newErrRegister(
	message string,
	id string,
	locale string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRegister, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldLocale, locale)
}
