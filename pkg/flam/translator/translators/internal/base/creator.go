package base

import (
	"errors"
	"fmt"

	"github.com/go-playground/locales"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
)

// NewCreator returns the [flam.ITranslatorCreator] of the configurations
// with the driver, creating translators of the locales returned by
// locale, which must return a new one on each call.
func NewCreator(
	driver string,
	locale func() locales.Translator,
) flam.ITranslatorCreator {
	return &creator{driver: driver, locale: locale}
}

// creator is the [flam.ITranslatorCreator] returned by [NewCreator].
type creator struct {
	driver string
	locale func() locales.Translator
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == c.driver
}

// Create returns the translator of the locale holding the texts of the
// translator.ConfigFieldTranslations field, failing at the first invalid
// one, and when a plural text misses a plural rule of the locale.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ITranslator, error) {
	id := bag.String(factory.ConfigFieldId)
	t, err := translator.NewTranslator(id, c.locale())
	if err != nil {
		return nil, err
	}

	if bag.Get(translator.ConfigFieldTranslations) == nil {
		return t, nil
	}
	translations := bag.Bag(translator.ConfigFieldTranslations)
	if translations == nil {
		return nil, flam.NewWrappedError(translator.ErrInvalidField, creatorCreateMessage).
			With(translator.ErrorFieldID, id).
			With(translator.ErrorFieldLocale, t.Locale()).
			With(translator.ErrorFieldField, translator.ConfigFieldTranslations)
	}

	l := &loader{translator: t}
	if err := l.load(translations, ""); err != nil {
		return nil, err
	}
	if err := t.VerifyTranslations(); err != nil {
		return nil, l.newErr("", err)
	}
	return t, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create translator"

// creatorRules returns the plural rules by the names the plural texts of
// the configuration give them.
func creatorRules() map[string]locales.PluralRule {
	return map[string]locales.PluralRule{
		"zero":  locales.PluralRuleZero,
		"one":   locales.PluralRuleOne,
		"two":   locales.PluralRuleTwo,
		"few":   locales.PluralRuleFew,
		"many":  locales.PluralRuleMany,
		"other": locales.PluralRuleOther,
	}
}

// Causes of the errors of the invalid texts of the configuration.
var (
	errValue = errors.New("not a string nor a bag")
	errKind  = errors.New("not a cardinal, ordinal nor range plural text")
	errRules = errors.New("not a bag of texts per plural rule")
	errRule  = errors.New("unknown plural rule")
	errText  = errors.New("not a string")
)

// loader adds the texts of a configuration to its translator.
type loader struct {
	translator flam.ITranslator
}

// load adds the texts of the bag, its keys prefixed with the prefix: a
// string is a text, a bag holding a plural text field is a plural text,
// and any other bag holds the texts of the keys it prefixes with its key
// and a dot.
func (l *loader) load(
	bag flam.IBag,
	prefix string,
) error {
	for key, val := range bag.All() {
		full := prefix + key
		if text, ok := val.(string); ok {
			if err := l.translator.Add(full, text, false); err != nil {
				return l.newErr(full, err)
			}
			continue
		}

		sub := bag.Bag(key)
		switch {
		case sub == nil:
			return l.newErr(full, errValue)
		case loaderPlural(sub):
			if err := l.loadPlural(sub, full); err != nil {
				return err
			}
		default:
			if err := l.load(sub, full+"."); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadPlural adds the plural texts of the key, held per plural rule name
// by the plural text fields of the bag.
func (l *loader) loadPlural(
	bag flam.IBag,
	key string,
) error {
	names := creatorRules()
	for kind := range bag.All() {
		var add func(key any, text string, rule locales.PluralRule, override bool) error
		switch kind {
		case translator.TranslationFieldCardinal:
			add = l.translator.AddCardinal
		case translator.TranslationFieldOrdinal:
			add = l.translator.AddOrdinal
		case translator.TranslationFieldRange:
			add = l.translator.AddRange
		default:
			return l.newErr(key, fmt.Errorf("%s: %w", kind, errKind))
		}

		rules := bag.Bag(kind)
		if rules == nil {
			return l.newErr(key, fmt.Errorf("%s: %w", kind, errRules))
		}
		for name, val := range rules.All() {
			rule, ok := names[name]
			if !ok {
				return l.newErr(key, fmt.Errorf("%s.%s: %w", kind, name, errRule))
			}
			text, ok := val.(string)
			if !ok {
				return l.newErr(key, fmt.Errorf("%s.%s: %w", kind, name, errText))
			}
			if err := add(key, text, rule, false); err != nil {
				return l.newErr(key, err)
			}
		}
	}
	return nil
}

// newErr returns the error wrapping translator.ErrTranslation, joined
// with the cause, with the creatorCreateMessage message and the id,
// locale and, when not empty, key context fields.
func (l *loader) newErr(
	key string,
	cause error,
) *flam.Error {
	e := flam.NewWrappedError(fmt.Errorf("%w: %w", translator.ErrTranslation, cause), creatorCreateMessage).
		With(translator.ErrorFieldID, l.translator.Id()).
		With(translator.ErrorFieldLocale, l.translator.Locale())
	if key != "" {
		e = e.With(translator.ErrorFieldKey, key)
	}
	return e
}

// loaderPlural reports whether the bag holds a plural text field, and so
// is a plural text.
func loaderPlural(
	bag flam.IBag,
) bool {
	return bag.Has(translator.TranslationFieldCardinal) ||
		bag.Has(translator.TranslationFieldOrdinal) ||
		bag.Has(translator.TranslationFieldRange)
}
