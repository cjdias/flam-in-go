package translator

import (
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/english"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/french"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/german"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/portuguese"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/spanish"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// driver describes a translator driver bundle.
type driver struct {
	name   string
	bundle func() flam.IBundle
	id     string
	driver string
	locale string
}

var drivers = []driver{
	{"english", english.NewBundle, english.Id, english.Driver, "en"},
	{"portuguese", portuguese.NewBundle, portuguese.Id, portuguese.Driver, "pt"},
	{"spanish", spanish.NewBundle, spanish.Id, spanish.Driver, "es"},
	{"french", french.NewBundle, french.Id, french.Driver, "fr"},
	{"german", german.NewBundle, german.Id, german.Driver, "de"},
}

// newDriverTranslator returns the translator of the id of an app with the
// translator bundles, the English translator configured with the
// translations, unless nil, and a translator "other" of an unknown
// driver.
func newDriverTranslator(t *testing.T, id string, translations any) (flam.ITranslator, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), translator.NewBundle(), english.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if translations != nil {
		testutil.NoError(t, bag.Set(translator.ConfigPath+".en."+translator.ConfigFieldTranslations, translations))
	}
	testutil.NoError(t, bag.Set(translator.ConfigPath+".other", container.Bag{factory.ConfigFieldDriver: "other"}))
	a := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var tr flam.ITranslator
	var err error
	testutil.NoError(t, a.Invoke(func(f flam.ITranslatorFactory) { tr, err = f.Get(id) }))
	return tr, err
}

// creatorError fails the test, and stops it, unless err is the error of
// the factory wrapping a creator *flam.Error that wraps the sentinel, with
// the text and exactly the context.
func creatorError(t *testing.T, err, sentinel error, text string, ctx map[string]any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(errors.Unwrap(err), &e) {
		t.Fatalf("Get() = %v, want it to wrap %v", err, sentinel)
	}
	testutil.Equal(t, "Error()", e.Error(), text)
	testutil.Context(t, e, ctx)
}

// translation returns the translation of the key by the translator.
func translation(t *testing.T, tr flam.ITranslator, key string, params ...string) string {
	t.Helper()
	got, err := tr.T(key, params...)
	testutil.NoError(t, err)
	return got
}

func Test_Drivers(t *testing.T) {
	for _, d := range drivers {
		t.Run("should configure the default "+d.name+" translator", func(t *testing.T) {
			b := d.bundle()
			testutil.Equal(t, "Id()", b.Id(), "flam.translator.translators."+d.name)
			testutil.Equal(t, "Id", d.id, d.locale)
			testutil.Equal(t, "Driver", d.driver, "flam.translator.translators.driver."+d.name)

			bag := container.Bag{}
			testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
			testutil.Equal(t, "config", bag.Get(translator.ConfigPath), container.Bag{
				d.locale: container.Bag{factory.ConfigFieldDriver: d.driver},
			})
			if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
				t.Fatal("Config() did not fail")
			}

			a := testutil.NewApp()
			testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
			testutil.NoError(t, a.RegisterBundle(translator.NewBundle()))
			testutil.NoError(t, a.RegisterBundle(b))
			calls := a.Calls()
			testutil.Equal(t, "calls", calls[len(calls)-2:], []string{"bundle:flam.translator.translators." + d.name, "provider/1"})
			testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
			testutil.NoError(t, a.Invoke(func(f flam.ITranslatorFactory) error {
				tr, err := f.Get(d.locale)
				if err != nil {
					return err
				}
				testutil.Equal(t, "Id()", tr.Id(), d.locale)
				testutil.Equal(t, "Locale()", tr.Locale(), d.locale)
				return nil
			}))
		})
	}

	t.Run("should register every translator driver", func(t *testing.T) {
		b := translators.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.translator.translators")
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		want := []string{"bundle:flam.translator.translators"}
		for _, d := range drivers {
			want = append(want, "bundle:flam.translator.translators."+d.name, "provider/1")
		}
		testutil.Equal(t, "calls", a.Calls(), want)
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := newDriverTranslator(t, "other", nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_Translations(t *testing.T) {
	t.Run("should hold no text without translations", func(t *testing.T) {
		tr, err := newDriverTranslator(t, "en", nil)
		testutil.NoError(t, err)
		if _, err := tr.T("welcome"); err == nil {
			t.Fatal("T() did not fail")
		}
	})

	t.Run("should load the translations", func(t *testing.T) {
		tr, err := newDriverTranslator(t, "en", container.Bag{
			"welcome": "Welcome, {0}!",
			"days": container.Bag{
				translator.TranslationFieldCardinal: container.Bag{"one": "{0} day", "other": "{0} days"},
				translator.TranslationFieldRange:    container.Bag{"other": "{0}-{1} days"},
			},
			"place": container.Bag{
				translator.TranslationFieldOrdinal: map[string]any{"one": "{0}st", "two": "{0}nd", "few": "{0}rd", "other": "{0}th"},
			},
			"errors": container.Bag{
				"required": "{0} is required",
				"nested":   container.Bag{"deep": "deep"},
			},
			"empty": container.Bag{},
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "welcome", translation(t, tr, "welcome", "Ann"), "Welcome, Ann!")
		testutil.Equal(t, "errors.required", translation(t, tr, "errors.required", "name"), "name is required")
		testutil.Equal(t, "errors.nested.deep", translation(t, tr, "errors.nested.deep"), "deep")
		got, err := tr.C("days", 1, 0, "1")
		testutil.NoError(t, err)
		testutil.Equal(t, "C()", got, "1 day")
		got, err = tr.R("days", 1, 0, 2, 0, "1", "2")
		testutil.NoError(t, err)
		testutil.Equal(t, "R()", got, "1-2 days")
		got, err = tr.O("place", 3, 0, "3")
		testutil.NoError(t, err)
		testutil.Equal(t, "O()", got, "3rd")
	})

	t.Run("should fail on translations that are not a bag", func(t *testing.T) {
		_, err := newDriverTranslator(t, "en", "scalar")
		creatorError(t, err, translator.ErrInvalidField, "unable to create translator: invalid translator field",
			map[string]any{translator.ErrorFieldID: "en", translator.ErrorFieldLocale: "en", translator.ErrorFieldField: "translations"})
	})

	for name, tc := range map[string]struct {
		translations container.Bag
		text         string
		key          string
	}{
		"a value that is not a string nor a bag": {
			container.Bag{"welcome": 7}, "not a string nor a bag", "welcome",
		},
		"an invalid nested value": {
			container.Bag{"errors": container.Bag{"required": true}}, "not a string nor a bag", "errors.required",
		},
		"an invalid text": {
			container.Bag{"welcome": "{0"}, "error: missing bracket '{}', in translation. locale: 'en' key: 'welcome' text: '{0'", "welcome",
		},
		"a field that is not a plural text field": {
			container.Bag{"days": container.Bag{"cardinal": container.Bag{"other": "{0}"}, "text": "x"}},
			"text: not a cardinal, ordinal nor range plural text", "days",
		},
		"plural texts that are not a bag": {
			container.Bag{"days": container.Bag{"cardinal": "x"}}, "cardinal: not a bag of texts per plural rule", "days",
		},
		"an unknown plural rule": {
			container.Bag{"days": container.Bag{"ordinal": container.Bag{"lots": "x"}}}, "ordinal.lots: unknown plural rule", "days",
		},
		"a plural text that is not a string": {
			container.Bag{"days": container.Bag{"range": container.Bag{"other": 1}}}, "range.other: not a string", "days",
		},
		"a plural rule the locale has not": {
			container.Bag{"days": container.Bag{"cardinal": container.Bag{"two": "{0}", "other": "{0}"}}}, "error: cardinal plural rule 'Two' does not exist for locale 'en' key: 'days' text: '{0}'", "days",
		},
	} {
		t.Run("should fail on "+name, func(t *testing.T) {
			_, err := newDriverTranslator(t, "en", tc.translations)
			creatorError(t, err, translator.ErrTranslation, "unable to create translator: invalid translation: "+tc.text,
				map[string]any{translator.ErrorFieldID: "en", translator.ErrorFieldLocale: "en", translator.ErrorFieldKey: tc.key})
		})
	}

	t.Run("should fail on a plural text missing a plural rule of the locale", func(t *testing.T) {
		_, err := newDriverTranslator(t, "en", container.Bag{
			"days": container.Bag{"cardinal": container.Bag{"one": "{0} day"}},
		})
		creatorError(t, err, translator.ErrTranslation,
			"unable to create translator: invalid translation: error: missing 'plural' plural rule 'Other' for translation with key 'days' and locale 'en'",
			map[string]any{translator.ErrorFieldID: "en", translator.ErrorFieldLocale: "en"})
	})
}
