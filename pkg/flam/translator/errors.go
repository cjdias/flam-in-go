package translator

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilLocale is the error wrapped by the error returned from
	// [NewTranslator] when the locale is nil.
	ErrNilLocale = errors.New("nil translator locale")

	// ErrInvalidKey is the error wrapped by the error returned from a
	// translator when a key cannot be compared, such as a slice, and so
	// cannot identify a text.
	ErrInvalidKey = errors.New("invalid translation key")

	// ErrMissingPlural is the error wrapped by the error returned from the
	// C, O and R methods of a translator when the key has plural texts,
	// but none for the plural rule of the number, as VerifyTranslations
	// reports.
	ErrMissingPlural = errors.New("missing plural translation")

	// ErrInvalidField is the error wrapped by the error returned from a
	// translator creator when the [ConfigFieldTranslations] field is not
	// a bag.
	ErrInvalidField = errors.New("invalid translator field")

	// ErrTranslation is the error wrapped, along with its cause, by the
	// error returned from a translator creator when a text of the
	// [ConfigFieldTranslations] field is invalid, or is rejected by the
	// translator, such as a plural text missing a plural rule of the
	// locale.
	ErrTranslation = errors.New("invalid translation")
)

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
