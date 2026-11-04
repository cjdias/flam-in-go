package parsers

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/de"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/es"
	"github.com/go-playground/locales/fr"
	"github.com/go-playground/locales/it"
	"github.com/go-playground/locales/pt"
	govalidator "github.com/go-playground/validator/v10"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/english"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/french"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/german"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// failingTranslator is a translator failing to add a text.
type failingTranslator struct{ flam.ITranslator }

func (failingTranslator) Add(any, string, bool) error { return errBoom }

type user struct {
	Name  string `validate:"required" paramId:"1" code:"11"`
	Email string `validate:"required,email" paramId:"x"`
	Age   int    `validate:"gte=18"`
}

type address struct {
	Street string `validate:"required" paramId:"7"`
}

type order struct {
	Address  address              `paramId:"3"`
	Billing  **address            `validate:"required"`
	Items    []item               `validate:"dive"`
	Tags     []string             `validate:"min=1,dive,required" paramId:"4"`
	Matrix   [][]item             `validate:"dive,dive"`
	Index    map[string]item      `validate:"dive"`
	Pointers map[string]*[]string `validate:"dive,dive,required"`
}

type item struct {
	Sku string `validate:"required" paramId:"5"`
}

// newTranslator returns a translator of the locale.
func newTranslator(t *testing.T, locale locales.Translator) flam.ITranslator {
	t.Helper()
	tr, err := translator.NewTranslator("main", locale)
	testutil.NoError(t, err)
	return tr
}

// newParser returns a parser of the annotation and the translator.
func newParser(t *testing.T, annotation string, tr flam.ITranslator) flam.IValidationParser {
	t.Helper()
	p, err := standard.NewParser("main", annotation, tr)
	testutil.NoError(t, err)
	return p
}

// parse returns the errors of the value, validated by a validate on which
// the parser registered.
func parse(t *testing.T, p flam.IValidationParser, value any) []flam.ValidationError {
	t.Helper()
	v := govalidator.New(govalidator.WithRequiredStructEnabled())
	testutil.NoError(t, p.Register(v))
	return p.Parse(value, errs(t, v, value))
}

// errs returns the errors of the value validated by the validate.
func errs(t *testing.T, v *govalidator.Validate, value any) govalidator.ValidationErrors {
	t.Helper()
	var verrs govalidator.ValidationErrors
	if !errors.As(v.Struct(value), &verrs) {
		t.Fatalf("Struct(%T) did not fail", value)
	}
	return verrs
}

// messages returns the messages of the errors.
func messages(errs []flam.ValidationError) []string {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.ErrorMessage
	}
	return msgs
}

// newDriverParser returns the parser of the id of an app with the
// translator and parser bundles, the configuration holding the values,
// the translator factory being provided unless without.
func newDriverParser(t *testing.T, id string, values map[string]any, without bool) (flam.IValidationParser, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), validator.NewBundle(), standard.NewBundle()}
	if !without {
		bundles = append(bundles, translator.NewBundle(), translators.NewBundle())
	}
	a := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	bag := container.Bag{}
	for _, b := range []flam.IBundle{standard.NewBundle(), english.NewBundle(), french.NewBundle(), german.NewBundle()} {
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var p flam.IValidationParser
	var err error
	testutil.NoError(t, a.Invoke(func(f flam.IValidationParserFactory) { p, err = f.Get(id) }))
	return p, err
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

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "Id", standard.Id, "standard")
	testutil.Equal(t, "Driver", standard.Driver, "flam.validator.parsers.driver.standard")
	testutil.Equal(t, "ConfigFieldAnnotation", standard.ConfigFieldAnnotation, "annotation")
	testutil.Equal(t, "ConfigFieldTranslator", standard.ConfigFieldTranslator, "translator")
	testutil.Equal(t, "DefaultTranslatorConfigPath", standard.DefaultTranslatorConfigPath, "flam.validator.defaults.parsers.standard.translator")
	testutil.Equal(t, "ErrorFieldID", standard.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", standard.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldTranslator", standard.ErrorFieldTranslator, "translator")
	testutil.Equal(t, "ErrorFieldLocale", standard.ErrorFieldLocale, "locale")
	for name, err := range map[string]error{
		"ErrInvalidField": standard.ErrInvalidField,
		"ErrTranslator":   standard.ErrTranslator,
		"ErrLocale":       standard.ErrLocale,
		"ErrNilValidate":  standard.ErrNilValidate,
		"ErrRegister":     standard.ErrRegister,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_TagCodes(t *testing.T) {
	codes := standard.TagCodes()
	testutil.Equal(t, "len", len(codes), 115)
	seen := map[int]string{}
	for tag, code := range codes {
		if code < 1 || code > 115 {
			t.Errorf("TagCodes()[%q] = %d, want it in [1, 115]", tag, code)
		}
		if other, ok := seen[code]; ok {
			t.Errorf("TagCodes()[%q] = TagCodes()[%q] = %d", tag, other, code)
		}
		seen[code] = tag
	}
	for tag, code := range map[string]int{"eqcsfield": 1, "cidr": 15, "alpha": 41, "base64": 57, "email": 63, "eq": 88, "gte": 90, "dir": 94, "min": 102, "required": 104, "unique": 115} {
		testutil.Equal(t, "TagCodes()["+tag+"]", codes[tag], code)
	}

	codes["required"] = 0
	testutil.Equal(t, "fresh map", standard.TagCodes()["required"], 104)
}

func Test_Bundles(t *testing.T) {
	t.Run("should configure the default parser", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator.parsers.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "parsers", bag.Get(validator.ParserConfigPath), container.Bag{
			standard.Id: container.Bag{factory.ConfigFieldDriver: standard.Driver},
		})
		testutil.Equal(t, "default parser", bag.Get(validator.DefaultParserConfigPath), any(standard.Id))
		for _, bag := range []container.Bag{
			{"flam": "scalar"},
			{"flam": container.Bag{"validator": container.Bag{"defaults": "scalar"}}},
		} {
			if err := b.(flam.IConfigurableBundle).Config(bag); err == nil {
				t.Fatalf("Config(%v) did not fail", bag)
			}
		}

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.validator.parsers.standard", "provider/1"})
	})

	t.Run("should register every parser driver", func(t *testing.T) {
		b := parsers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator.parsers")
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.validator.parsers", "bundle:flam.validator.parsers.standard", "provider/1",
		})
	})
}

func Test_NewParser(t *testing.T) {
	t.Run("should leave the messages untranslated without translator", func(t *testing.T) {
		var nilTranslator flam.ITranslator = (*struct{ flam.ITranslator })(nil)
		for _, tr := range []flam.ITranslator{nil, nilTranslator} {
			p := newParser(t, "paramId", tr)
			testutil.Equal(t, "Id()", p.Id(), "main")
			testutil.Equal(t, "Parse()", parse(t, p, user{Name: "ann", Email: "ann@example.com"}), []flam.ValidationError{{
				ParamName:    "Age",
				ErrorId:      90,
				ErrorMessage: "Key: 'user.Age' Error:Field validation for 'Age' failed on the 'gte' tag",
			}})
		}
	})

	t.Run("should translate the messages into the locale of the translator", func(t *testing.T) {
		for locale, want := range map[locales.Translator][]string{
			en.New(): {"Name is a required field", "Email must be a valid email address", "Age must be 18 or greater"},
			pt.New(): {"Name é obrigatório", "Email deve ser um endereço de e-mail válido", "Age deve ser maior ou igual a 18"},
			es.New(): {"Name es un campo requerido", "Email debe ser una dirección de correo electrónico válida", "Age debe ser 18 o mayor"},
			fr.New(): {"Name est un champ obligatoire", "Email doit être une adresse email valide", "Age doit être 18 ou plus"},
			de.New(): {"Name ist ein Pflichtfeld", "Email muss eine gültige E-Mail-Adresse sein", "Age muss 18 oder größer sein"},
		} {
			t.Run(locale.Locale(), func(t *testing.T) {
				p := newParser(t, "paramId", newTranslator(t, locale))
				testutil.Equal(t, "messages", messages(parse(t, p, user{Email: "ann"})), want)
			})
		}
	})

	t.Run("should fail on a locale without messages", func(t *testing.T) {
		p, err := standard.NewParser("main", "paramId", newTranslator(t, it.New()))
		testutil.Error(t, err, standard.ErrLocale, "unable to create standard validation parser: unsupported standard validation parser locale",
			map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldLocale: "it"})
		if p != nil {
			t.Fatalf("NewParser() = %v, want nil", p)
		}
	})
}

func Test_Register(t *testing.T) {
	t.Run("should fail on a nil validate", func(t *testing.T) {
		for _, tr := range []flam.ITranslator{nil, newTranslator(t, en.New())} {
			err := newParser(t, "paramId", tr).Register(nil)
			testutil.Error(t, err, standard.ErrNilValidate, "unable to register standard validation parser messages: nil standard validation parser validate",
				map[string]any{standard.ErrorFieldID: "main"})
		}
	})

	t.Run("should keep the texts of the translator", func(t *testing.T) {
		tr := newTranslator(t, en.New())
		testutil.NoError(t, tr.Add("required", "{0} is missing", false))
		p := newParser(t, "paramId", tr)
		testutil.Equal(t, "messages", messages(parse(t, p, user{Email: "ann@example.com", Age: 18})), []string{"Name is missing"})
		got, err := tr.T("email", "Email")
		testutil.NoError(t, err)
		testutil.Equal(t, "added text", got, "Email must be a valid email address")
	})

	t.Run("should register on several validates", func(t *testing.T) {
		p := newParser(t, "paramId", newTranslator(t, en.New()))
		for range 2 {
			testutil.Equal(t, "messages", messages(parse(t, p, user{Email: "ann@example.com", Age: 18})), []string{"Name is a required field"})
		}
	})

	t.Run("should keep the parsers of a translator apart", func(t *testing.T) {
		tr := newTranslator(t, en.New())
		first, second := newParser(t, "paramId", tr), newParser(t, "paramId", tr)
		v := govalidator.New()
		testutil.NoError(t, first.Register(v))
		value := user{Email: "ann@example.com", Age: 18}
		testutil.Equal(t, "first", messages(first.Parse(value, errs(t, v, value))), []string{"Name is a required field"})
		testutil.Equal(t, "second", messages(second.Parse(value, errs(t, v, value))), []string{
			"Key: 'user.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		})
		testutil.Equal(t, "second registered", messages(parse(t, second, value)), []string{"Name is a required field"})
	})

	t.Run("should fail when the translator refuses a text", func(t *testing.T) {
		p := newParser(t, "paramId", failingTranslator{newTranslator(t, en.New())})
		err := p.Register(govalidator.New())
		testutil.Error(t, err, standard.ErrRegister, "unable to register standard validation parser messages: unable to register standard validation parser messages: boom",
			map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldLocale: "en"})
		if !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want it to wrap errBoom", err)
		}
	})
}

func Test_Parse(t *testing.T) {
	t.Run("should describe the errors of the fields", func(t *testing.T) {
		p := newParser(t, "paramId", nil)
		got := parse(t, p, &user{Email: "ann"})
		for i, want := range []flam.ValidationError{
			{ParamId: 1, ParamName: "Name", ErrorId: 104},
			{ParamId: 0, ParamName: "Email", ErrorId: 63},
			{ParamId: 0, ParamName: "Age", ErrorId: 90},
		} {
			want.ErrorMessage = got[i].ErrorMessage
			testutil.Equal(t, fmt.Sprintf("Parse()[%d]", i), got[i], want)
		}
		testutil.Equal(t, "len", len(got), 3)
	})

	t.Run("should read the ids from the annotation", func(t *testing.T) {
		p := newParser(t, "code", nil)
		got := parse(t, p, user{Email: "ann@example.com", Age: 18})
		testutil.Equal(t, "ParamId", got[0].ParamId, 11)
		got = parse(t, newParser(t, "", nil), user{Email: "ann@example.com", Age: 18})
		testutil.Equal(t, "ParamId", got[0].ParamId, 0)
	})

	t.Run("should follow the path of nested fields", func(t *testing.T) {
		p := newParser(t, "paramId", nil)
		sku := item{Sku: "a"}
		got := parse(t, p, order{
			Items:    []item{sku, {}},
			Tags:     []string{"", "a"},
			Matrix:   [][]item{{sku}, {sku, {}}},
			Index:    map[string]item{"k": {}},
			Pointers: map[string]*[]string{"p": {""}},
		})
		want := []flam.ValidationError{
			{ParamId: 7, ParamName: "Address.Street", ErrorId: 104},
			{ParamId: 0, ParamName: "Billing", ErrorId: 104},
			{ParamId: 5, ParamName: "Items[1].Sku", ErrorId: 104},
			{ParamId: 4, ParamName: "Tags[0]", ErrorId: 104},
			{ParamId: 5, ParamName: "Matrix[1][1].Sku", ErrorId: 104},
			{ParamId: 5, ParamName: "Index[k].Sku", ErrorId: 104},
			{ParamId: 0, ParamName: "Pointers[p][0]", ErrorId: 104},
		}
		testutil.Equal(t, "len", len(got), len(want))
		for i := range want {
			want[i].ErrorMessage = got[i].ErrorMessage
			testutil.Equal(t, fmt.Sprintf("Parse()[%d]", i), got[i], want[i])
		}

		billing := &address{}
		got = parse(t, p, order{Address: address{Street: "a"}, Billing: &billing, Tags: []string{"a"}})
		testutil.Equal(t, "Billing.Street", got[0].ParamName, "Billing.Street")
		testutil.Equal(t, "Billing.Street", got[0].ParamId, 7)
	})

	t.Run("should take no id from a value of another type", func(t *testing.T) {
		p := newParser(t, "paramId", nil)
		v := govalidator.New()
		verrs := errs(t, v, order{Address: address{Street: "a"}, Items: []item{{}}, Tags: []string{"a"}})
		type mismatch struct {
			Billing int
			Items   string
		}
		for _, value := range []any{nil, 7, []int{}, struct{}{}, mismatch{}, &mismatch{}} {
			got := p.Parse(value, verrs)
			testutil.Equal(t, "names", []string{got[0].ParamName, got[1].ParamName}, []string{"Billing", "Items[0].Sku"})
			testutil.Equal(t, "ids", []int{got[0].ParamId, got[1].ParamId}, []int{0, 0})
		}
	})

	t.Run("should give the codes of the added, unknown and aliased rules", func(t *testing.T) {
		p := newParser(t, "paramId", nil)
		v := govalidator.New()
		testutil.NoError(t, v.RegisterValidation("custom", func(govalidator.FieldLevel) bool { return false }))
		testutil.NoError(t, v.RegisterValidation("unknown", func(govalidator.FieldLevel) bool { return false }))
		v.RegisterAlias("present", "required")
		value := struct {
			Custom  string `validate:"custom"`
			Unknown string `validate:"unknown"`
			Present string `validate:"present"`
		}{}
		p.AddTagCode("custom", 900)
		got := p.Parse(value, errs(t, v, value))
		testutil.Equal(t, "codes", []int{got[0].ErrorId, got[1].ErrorId, got[2].ErrorId}, []int{900, 0, 104})
		p.AddTagCode("required", 1)
		testutil.Equal(t, "replaced code", p.Parse(value, errs(t, v, value))[2].ErrorId, 1)
	})

	t.Run("should parse no error", func(t *testing.T) {
		testutil.Equal(t, "Parse()", newParser(t, "paramId", nil).Parse(user{}, nil), []flam.ValidationError{})
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		p := newParser(t, "paramId", newTranslator(t, en.New()))
		v := govalidator.New()
		testutil.NoError(t, p.Register(v))
		value := user{Email: "ann@example.com", Age: 18}
		verrs := errs(t, v, value)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				p.AddTagCode(fmt.Sprintf("tag%d", i), i)
				testutil.Equal(t, "messages", messages(p.Parse(value, verrs)), []string{"Name is a required field"})
			})
		}
		wg.Wait()
	})
}

func Test_Creator(t *testing.T) {
	value := user{Email: "ann@example.com", Age: 18}

	t.Run("should create the default parser untranslated", func(t *testing.T) {
		p, err := newDriverParser(t, standard.Id, nil, true)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", p.Id(), standard.Id)
		testutil.Equal(t, "Parse()", parse(t, p, value), []flam.ValidationError{{
			ParamId:      1,
			ParamName:    "Name",
			ErrorId:      104,
			ErrorMessage: "Key: 'user.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		}})
	})

	t.Run("should take the default translator", func(t *testing.T) {
		p, err := newDriverParser(t, standard.Id, map[string]any{standard.DefaultTranslatorConfigPath: "de"}, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "messages", messages(parse(t, p, value)), []string{"Name ist ein Pflichtfeld"})
	})

	t.Run("should take the fields of the configuration", func(t *testing.T) {
		p, err := newDriverParser(t, "other", map[string]any{
			standard.DefaultTranslatorConfigPath: "de",
			validator.ParserConfigPath + ".other": container.Bag{
				factory.ConfigFieldDriver:      standard.Driver,
				standard.ConfigFieldAnnotation: "code",
				standard.ConfigFieldTranslator: "fr",
			},
		}, false)
		testutil.NoError(t, err)
		got := parse(t, p, value)
		testutil.Equal(t, "ParamId", got[0].ParamId, 11)
		testutil.Equal(t, "messages", messages(got), []string{"Name est un champ obligatoire"})
	})

	t.Run("should leave the messages untranslated with an empty translator", func(t *testing.T) {
		p, err := newDriverParser(t, "other", map[string]any{
			standard.DefaultTranslatorConfigPath: "de",
			validator.ParserConfigPath + ".other": container.Bag{
				factory.ConfigFieldDriver:      standard.Driver,
				standard.ConfigFieldTranslator: "",
			},
		}, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "messages", messages(parse(t, p, value)), []string{
			"Key: 'user.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		})
	})

	for name, tc := range map[string]struct {
		values map[string]any
		field  string
	}{
		"an annotation that is not a string": {
			map[string]any{validator.ParserConfigPath + ".standard." + standard.ConfigFieldAnnotation: 7},
			standard.ConfigFieldAnnotation,
		},
		"a translator that is not a string": {
			map[string]any{validator.ParserConfigPath + ".standard." + standard.ConfigFieldTranslator: 7},
			standard.ConfigFieldTranslator,
		},
		"a default translator that is not a string": {
			map[string]any{standard.DefaultTranslatorConfigPath: 7},
			standard.ConfigFieldTranslator,
		},
	} {
		t.Run("should fail on "+name, func(t *testing.T) {
			_, err := newDriverParser(t, standard.Id, tc.values, false)
			creatorError(t, err, standard.ErrInvalidField, "unable to create standard validation parser: invalid standard validation parser field",
				map[string]any{standard.ErrorFieldID: standard.Id, standard.ErrorFieldField: tc.field})
		})
	}

	t.Run("should fail on an unknown translator", func(t *testing.T) {
		_, err := newDriverParser(t, standard.Id, map[string]any{standard.DefaultTranslatorConfigPath: "xx"}, false)
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want it to wrap factory.ErrUnknownID", err)
		}
		creatorError(t, err, standard.ErrTranslator, errors.Unwrap(err).Error(),
			map[string]any{standard.ErrorFieldID: standard.Id, standard.ErrorFieldTranslator: "xx"})
	})

	t.Run("should fail without translator factory", func(t *testing.T) {
		_, err := newDriverParser(t, standard.Id, map[string]any{standard.DefaultTranslatorConfigPath: "en"}, true)
		creatorError(t, err, standard.ErrTranslator,
			"unable to create standard validation parser: unable to get standard validation parser translator: no translator factory",
			map[string]any{standard.ErrorFieldID: standard.Id, standard.ErrorFieldTranslator: "en"})
	})
}
