package translator

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/pt"
	ut "github.com/go-playground/universal-translator"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// failing is a translator failing to close.
type failing struct{ flam.ITranslator }

func (failing) Id() string   { return "bad" }
func (failing) Close() error { return errBoom }

// newTranslator returns the translator of the id over the English locale.
func newTranslator(t *testing.T) flam.ITranslator {
	t.Helper()
	tr, err := translator.NewTranslator("main", en.New())
	testutil.NoError(t, err)
	return tr
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ConfigPath", translator.ConfigPath, "flam.translator.translators")
	testutil.Equal(t, "CreatorGroup", translator.CreatorGroup, "flam.translator.translators.creator")
	testutil.Equal(t, "ConfigFieldTranslations", translator.ConfigFieldTranslations, "translations")
	testutil.Equal(t, "TranslationFieldCardinal", translator.TranslationFieldCardinal, "cardinal")
	testutil.Equal(t, "TranslationFieldOrdinal", translator.TranslationFieldOrdinal, "ordinal")
	testutil.Equal(t, "TranslationFieldRange", translator.TranslationFieldRange, "range")
	testutil.Equal(t, "ErrorFieldID", translator.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldLocale", translator.ErrorFieldLocale, "locale")
	testutil.Equal(t, "ErrorFieldKey", translator.ErrorFieldKey, "key")
	testutil.Equal(t, "ErrorFieldField", translator.ErrorFieldField, "field")
	for name, err := range map[string]error{
		"ErrNilLocale":     translator.ErrNilLocale,
		"ErrInvalidKey":    translator.ErrInvalidKey,
		"ErrMissingPlural": translator.ErrMissingPlural,
		"ErrInvalidField":  translator.ErrInvalidField,
		"ErrTranslation":   translator.ErrTranslation,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factory", func(t *testing.T) {
		b := translator.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.translator")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(b))
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-2:], []string{"bundle:flam.translator", "provider/0"})
		testutil.NoError(t, a.Invoke(func(f flam.ITranslatorFactory) {
			testutil.Equal(t, "Available()", f.Available(), []string{})
		}))
	})

	t.Run("should close the factory", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(translator.NewBundle()))
		var f flam.ITranslatorFactory
		testutil.NoError(t, a.Invoke(func(tf flam.ITranslatorFactory) { f = tf }))
		testutil.NoError(t, f.Store("bad", failing{}))

		if err := translator.NewBundle().(flam.IClosableBundle).Close(a); !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
		testutil.Equal(t, "Has()", f.Has("bad"), false)
		testutil.NoError(t, translator.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to close without the factory dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(translator.NewBundle()))
		if err := translator.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}

func Test_NewTranslator(t *testing.T) {
	t.Run("should fail on a nil locale", func(t *testing.T) {
		for name, locale := range map[string]locales.Translator{"nil": nil, "typed nil": (*nilLocale)(nil)} {
			tr, err := translator.NewTranslator("main", locale)
			testutil.Error(t, err, translator.ErrNilLocale, "unable to create translator: nil translator locale",
				map[string]any{translator.ErrorFieldID: "main"})
			testutil.Equal(t, name+" translator", tr, nil)
		}
	})

	t.Run("should return a translator of the locale, holding no text", func(t *testing.T) {
		tr := newTranslator(t)
		testutil.Equal(t, "Id()", tr.Id(), "main")
		testutil.Equal(t, "Locale()", tr.Locale(), "en")
		testutil.Equal(t, "FmtNumber()", tr.FmtNumber(1234.5, 1), "1,234.5")
		_, err := tr.T("welcome")
		if !errors.Is(err, ut.ErrUnknownTranslation) {
			t.Fatalf("T() = %v, want ut.ErrUnknownTranslation", err)
		}
		testutil.NoError(t, tr.VerifyTranslations())
		testutil.NoError(t, tr.Add("welcome", "hi", false))
	})

	t.Run("should return translators not sharing texts", func(t *testing.T) {
		a, b := newTranslator(t), newTranslator(t)
		testutil.NoError(t, a.Add("welcome", "hi", false))
		if _, err := b.T("welcome"); err == nil {
			t.Fatal("T() did not fail")
		}
	})
}

// nilLocale is a locale type of which a nil pointer is a non-nil
// locales.Translator.
type nilLocale struct{ locales.Translator }

func Test_Translator(t *testing.T) {
	t.Run("should translate the texts", func(t *testing.T) {
		tr := newTranslator(t)
		testutil.NoError(t, tr.Add("welcome", "Welcome, {0}!", false))
		testutil.NoError(t, tr.AddCardinal("days", "{0} day", locales.PluralRuleOne, false))
		testutil.NoError(t, tr.AddCardinal("days", "{0} days", locales.PluralRuleOther, false))
		for rule, text := range map[locales.PluralRule]string{
			locales.PluralRuleOne: "{0}st", locales.PluralRuleTwo: "{0}nd",
			locales.PluralRuleFew: "{0}rd", locales.PluralRuleOther: "{0}th",
		} {
			testutil.NoError(t, tr.AddOrdinal("place", text, rule, false))
		}
		testutil.NoError(t, tr.AddRange("span", "{0}-{1} days", locales.PluralRuleOther, false))
		testutil.NoError(t, tr.VerifyTranslations())

		got, err := tr.T("welcome", "Ann")
		testutil.NoError(t, err)
		testutil.Equal(t, "T()", got, "Welcome, Ann!")
		got, err = tr.C("days", 1, 0, "1")
		testutil.NoError(t, err)
		testutil.Equal(t, "C(1)", got, "1 day")
		got, err = tr.C("days", 2, 0, "2")
		testutil.NoError(t, err)
		testutil.Equal(t, "C(2)", got, "2 days")
		got, err = tr.O("place", 2, 0, "2")
		testutil.NoError(t, err)
		testutil.Equal(t, "O(2)", got, "2nd")
		got, err = tr.R("span", 1, 0, 3, 0, "1", "3")
		testutil.NoError(t, err)
		testutil.Equal(t, "R()", got, "1-3 days")
	})

	t.Run("should keep the go-playground errors", func(t *testing.T) {
		tr := newTranslator(t)
		testutil.NoError(t, tr.Add("welcome", "hi", false))
		if err := tr.Add("welcome", "hello", false); !errors.As(err, new(*ut.ErrConflictingTranslation)) {
			t.Fatalf("Add() = %v, want ut.ErrConflictingTranslation", err)
		}
		testutil.NoError(t, tr.Add("welcome", "hello", true))
		got, err := tr.T("welcome")
		testutil.NoError(t, err)
		testutil.Equal(t, "T()", got, "hello")

		testutil.NoError(t, tr.AddCardinal("days", "{0} day", locales.PluralRuleOne, false))
		if err := tr.VerifyTranslations(); !errors.As(err, new(*ut.ErrMissingPluralTranslation)) {
			t.Fatalf("VerifyTranslations() = %v, want ut.ErrMissingPluralTranslation", err)
		}
	})

	t.Run("should fail on a plural text missing the plural rule of the number", func(t *testing.T) {
		tr := newTranslator(t)
		testutil.NoError(t, tr.AddCardinal("days", "{0} days", locales.PluralRuleOther, false))
		testutil.NoError(t, tr.AddOrdinal("place", "{0}th", locales.PluralRuleOther, false))
		testutil.NoError(t, tr.AddRange("span", "{0}-{1}", locales.PluralRuleOther, false))
		want := func(text string, err error, key string) {
			t.Helper()
			testutil.Equal(t, "text", text, "")
			testutil.Error(t, err, translator.ErrMissingPlural,
				"unable to translate plural text: missing plural translation",
				map[string]any{translator.ErrorFieldID: "main", translator.ErrorFieldLocale: "en", translator.ErrorFieldKey: key})
		}
		testutil.NotPanics(t, "translator", func() {
			text, err := tr.C("days", 1, 0, "1")
			want(text, err, "days")
			text, err = tr.O("place", 1, 0, "1")
			want(text, err, "place")
		})
		got, err := tr.C("days", 2, 0, "2")
		testutil.NoError(t, err)
		testutil.Equal(t, "C(2)", got, "2 days")
		got, err = tr.R("span", 1, 0, 2, 0, "1", "2")
		testutil.NoError(t, err)
		testutil.Equal(t, "R()", got, "1-2")
	})

	t.Run("should take any comparable key", func(t *testing.T) {
		type key struct{ name string }
		tr := newTranslator(t)
		testutil.NoError(t, tr.Add(key{"welcome"}, "hi", false))
		testutil.NoError(t, tr.Add(nil, "none", false))
		got, err := tr.T(key{"welcome"})
		testutil.NoError(t, err)
		testutil.Equal(t, "T()", got, "hi")
		got, err = tr.T(nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "T(nil)", got, "none")
	})

	t.Run("should fail on a key that cannot be compared", func(t *testing.T) {
		tr := newTranslator(t)
		key := []string{"a"}
		want := func(err error) {
			t.Helper()
			testutil.Error(t, err, translator.ErrInvalidKey,
				"unable to use translation key: invalid translation key",
				map[string]any{translator.ErrorFieldID: "main", translator.ErrorFieldLocale: "en", translator.ErrorFieldKey: "[a]"})
		}
		testutil.NotPanics(t, "translator", func() {
			want(tr.Add(key, "x", false))
			want(tr.AddCardinal(key, "x", locales.PluralRuleOther, false))
			want(tr.AddOrdinal(key, "x", locales.PluralRuleOther, false))
			want(tr.AddRange(key, "x", locales.PluralRuleOther, false))
			_, err := tr.T(key)
			want(err)
			_, err = tr.C(key, 1, 0, "1")
			want(err)
			_, err = tr.O(key, 1, 0, "1")
			want(err)
			_, err = tr.R(key, 1, 0, 2, 0, "1", "2")
			want(err)
		})
	})

	t.Run("should be safe to add texts while translating", func(t *testing.T) {
		tr := newTranslator(t)
		testutil.NoError(t, tr.Add("welcome", "hi", false))
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				for j := range 100 {
					key := fmt.Sprintf("k%d.%d", i, j)
					_ = tr.Add(key, "x", false)
					_ = tr.AddCardinal(key, "{0}", locales.PluralRuleOther, false)
					_ = tr.AddOrdinal(key, "{0}", locales.PluralRuleOther, false)
					_ = tr.AddRange(key, "{0}-{1}", locales.PluralRuleOther, false)
					_, _ = tr.T("welcome")
					_, _ = tr.C(key, 1, 0, "1")
					_, _ = tr.O(key, 1, 0, "1")
					_, _ = tr.R(key, 1, 0, 2, 0, "1", "2")
					_ = tr.VerifyTranslations()
				}
			})
		}
		wg.Wait()
	})
}

func Test_Application(t *testing.T) {
	values := container.Bag{}
	testutil.NoError(t, values.Set(translator.ConfigPath+".pt.translations", container.Bag{"welcome": "Bem-vindo, {0}!"}))
	a := app.New(values)
	testutil.NoError(t, a.RegisterBundle(translator.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(translators.NewBundle()))
	testutil.NoError(t, a.Boot())

	testutil.NoError(t, a.Invoke(func(f flam.ITranslatorFactory) error {
		testutil.Equal(t, "Available()", f.Available(), []string{"de", "en", "es", "fr", "pt"})
		tr, err := f.Get("pt")
		if err != nil {
			return err
		}
		testutil.Equal(t, "Locale()", tr.Locale(), pt.New().Locale())
		got, err := tr.T("welcome", "Ana")
		testutil.Equal(t, "T()", got, "Bem-vindo, Ana!")
		return err
	}))
	testutil.NoError(t, a.Close())
}
