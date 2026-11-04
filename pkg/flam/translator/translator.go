package translator

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/go-playground/locales"
	ut "github.com/go-playground/universal-translator"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewTranslator returns the [flam.ITranslator] of the id over a new
// go-playground translator of the locale, holding no text, for the
// translator creators, such as:
//
//	translator.NewTranslator(id, en.New())
//
// It fails with ErrNilLocale when the locale is nil. The translator may
// be used from several goroutines. Its methods fail with ErrInvalidKey
// for a key that cannot be compared, and C, O and R with ErrMissingPlural
// for a key missing the text of the plural rule of the number, which
// go-playground would panic on.
func NewTranslator(
	id string,
	locale locales.Translator,
) (flam.ITranslator, error) {
	if value.IsNil(locale) {
		return nil, newErrNil(ErrNilLocale, translatorNewMessage, id)
	}
	return &translator{
		Translator: ut.New(locale, locale).GetFallback(),
		id:         id,
	}, nil
}

// translator is the [flam.ITranslator] returned by [NewTranslator], its
// embedded go-playground translator answering the locale methods, which
// read only the locale, while mu guards its texts.
type translator struct {
	ut.Translator

	id string
	mu sync.RWMutex
}

// compile time check that *translator implements ITranslator
var _ flam.ITranslator = (*translator)(nil)

func (t *translator) Id() string {
	return t.id
}

func (t *translator) Add(
	key any,
	text string,
	override bool,
) error {
	if err := t.checkKey(key); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Translator.Add(key, text, override)
}

func (t *translator) AddCardinal(
	key any,
	text string,
	rule locales.PluralRule,
	override bool,
) error {
	if err := t.checkKey(key); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Translator.AddCardinal(key, text, rule, override)
}

func (t *translator) AddOrdinal(
	key any,
	text string,
	rule locales.PluralRule,
	override bool,
) error {
	if err := t.checkKey(key); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Translator.AddOrdinal(key, text, rule, override)
}

func (t *translator) AddRange(
	key any,
	text string,
	rule locales.PluralRule,
	override bool,
) error {
	if err := t.checkKey(key); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Translator.AddRange(key, text, rule, override)
}

func (t *translator) T(
	key any,
	params ...string,
) (string, error) {
	if err := t.checkKey(key); err != nil {
		return "", err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Translator.T(key, params...)
}

func (t *translator) C(
	key any,
	num float64,
	digits uint64,
	param string,
) (string, error) {
	return t.plural(key, func() (string, error) {
		return t.Translator.C(key, num, digits, param)
	})
}

func (t *translator) O(
	key any,
	num float64,
	digits uint64,
	param string,
) (string, error) {
	return t.plural(key, func() (string, error) {
		return t.Translator.O(key, num, digits, param)
	})
}

func (t *translator) R(
	key any,
	num1 float64,
	digits1 uint64,
	num2 float64,
	digits2 uint64,
	param1 string,
	param2 string,
) (string, error) {
	return t.plural(key, func() (string, error) {
		return t.Translator.R(key, num1, digits1, num2, digits2, param1, param2)
	})
}

func (t *translator) VerifyTranslations() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Translator.VerifyTranslations()
}

// Messages of the errors returned by NewTranslator and the translator.
const (
	translatorNewMessage    = "unable to create translator"
	translatorKeyMessage    = "unable to use translation key"
	translatorPluralMessage = "unable to translate plural text"
)

// plural returns the plural text of the key translated by translate, the
// C, O or R method of the go-playground translator, which panics when the
// key has no text for the plural rule of the number, failing then with
// ErrMissingPlural.
func (t *translator) plural(
	key any,
	translate func() (string, error),
) (text string, err error) {
	if err := t.checkKey(key); err != nil {
		return "", err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	defer func() {
		if recover() != nil {
			text, err = "", t.newErr(ErrMissingPlural, translatorPluralMessage, key)
		}
	}()
	return translate()
}

// checkKey fails with ErrInvalidKey when the key cannot be compared, and
// so cannot be a key of the go-playground maps.
func (t *translator) checkKey(
	key any,
) error {
	if key == nil || reflect.ValueOf(key).Comparable() {
		return nil
	}
	return t.newErr(ErrInvalidKey, translatorKeyMessage, key)
}

// newErr returns the error wrapping the sentinel with the message and the
// ErrorFieldID, ErrorFieldLocale and ErrorFieldKey context fields.
func (t *translator) newErr(
	sentinel error,
	message string,
	key any,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, t.id).
		With(ErrorFieldLocale, t.Locale()).
		With(ErrorFieldKey, fmt.Sprintf("%v", key))
}
