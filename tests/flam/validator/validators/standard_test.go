package validators

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	govalidator "github.com/go-playground/validator/v10"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers"
	parserstandard "github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/validators"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/validators/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// parser is a parser recording the validates it registers on, failing to
// register with err, unless nil.
type parser struct {
	flam.IValidationParser
	err       error
	validates []*govalidator.Validate
}

func (p *parser) Register(v *govalidator.Validate) error {
	p.validates = append(p.validates, v)
	return p.err
}

// converter is a converter keeping the names of the fields.
type converter struct{}

func (converter) Id() string { return "names" }

func (converter) Convert(errs []flam.ValidationError) any {
	names := make([]string, len(errs))
	for i, e := range errs {
		names[i] = e.ParamName
	}
	return names
}

// converterCreator creates the converter.
type converterCreator struct{}

func (converterCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "names"
}

func (converterCreator) Create(flam.IBag) (flam.IValidationErrorConverter, error) {
	return converter{}, nil
}

type inner struct {
	Value string
}

type user struct {
	Name  string `validate:"required" paramId:"1"`
	Inner inner  `validate:"required"`
}

// newParser returns an untranslated standard parser.
func newParser(t *testing.T) flam.IValidationParser {
	t.Helper()
	p, err := parserstandard.NewParser("parser", "paramId", nil)
	testutil.NoError(t, err)
	return p
}

// newValidator returns a validator of the parser and the converter.
func newValidator(t *testing.T, p flam.IValidationParser, c flam.IValidationErrorConverter) flam.IValidator {
	t.Helper()
	v, err := standard.NewValidator("main", p, c)
	testutil.NoError(t, err)
	return v
}

// newDriverValidator returns the validator of the id of an app with the
// validator, parser and validator bundles and a converter "names", the
// configuration holding the values, but the default parser when
// withoutParser.
func newDriverValidator(t *testing.T, id string, values map[string]any, withoutParser bool) (flam.IValidator, error) {
	t.Helper()
	a := testutil.NewApp()
	for _, b := range []flam.IBundle{config.NewBundle(), validator.NewBundle(), parsers.NewBundle(), standard.NewBundle()} {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Provide(func() flam.IValidationErrorConverterCreator { return converterCreator{} },
		dig.Group(validator.ConverterCreatorGroup)))

	bag := container.Bag{}
	configured := []flam.IBundle{standard.NewBundle()}
	if !withoutParser {
		configured = append(configured, parserstandard.NewBundle())
	}
	for _, b := range configured {
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	}
	testutil.NoError(t, bag.Set(validator.ConverterConfigPath+".names", container.Bag{factory.ConfigFieldDriver: "names"}))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var v flam.IValidator
	var err error
	testutil.NoError(t, a.Invoke(func(f flam.IValidatorFactory) { v, err = f.Get(id) }))
	return v, err
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
	testutil.Equal(t, "Driver", standard.Driver, "flam.validator.validators.driver.standard")
	testutil.Equal(t, "ErrorFieldID", standard.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", standard.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldParser", standard.ErrorFieldParser, "parser")
	testutil.Equal(t, "ErrorFieldConverter", standard.ErrorFieldConverter, "converter")
	for name, err := range map[string]error{
		"ErrInvalidField": standard.ErrInvalidField,
		"ErrParser":       standard.ErrParser,
		"ErrConverter":    standard.ErrConverter,
		"ErrNilParser":    standard.ErrNilParser,
		"ErrRegister":     standard.ErrRegister,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundles(t *testing.T) {
	t.Run("should configure the default validator", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator.validators.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "validators", bag.Get(validator.ValidatorConfigPath), container.Bag{
			standard.Id: container.Bag{factory.ConfigFieldDriver: standard.Driver},
		})
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.validator.validators.standard", "provider/1"})
	})

	t.Run("should register every validator driver", func(t *testing.T) {
		b := validators.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator.validators")
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.validator.validators", "bundle:flam.validator.validators.standard", "provider/1",
		})
	})
}

func Test_NewValidator(t *testing.T) {
	t.Run("should fail on a nil parser", func(t *testing.T) {
		for _, p := range []flam.IValidationParser{nil, (*parser)(nil)} {
			v, err := standard.NewValidator("main", p, nil)
			testutil.Error(t, err, standard.ErrNilParser, "unable to create standard validator: nil standard validator parser",
				map[string]any{standard.ErrorFieldID: "main"})
			if v != nil {
				t.Fatalf("NewValidator() = %v, want nil", v)
			}
		}
	})

	t.Run("should fail when the parser fails to register", func(t *testing.T) {
		v, err := standard.NewValidator("main", &parser{err: errBoom}, nil)
		testutil.Error(t, err, standard.ErrRegister, "unable to create standard validator: unable to register standard validator parser: boom",
			map[string]any{standard.ErrorFieldID: "main"})
		if !errors.Is(err, errBoom) || v != nil {
			t.Fatalf("NewValidator() = %v, %v, want nil and errBoom", v, err)
		}
	})

	t.Run("should register the parser on a validate of its own", func(t *testing.T) {
		p := &parser{}
		first := newValidator(t, p, nil)
		newValidator(t, p, nil)
		testutil.Equal(t, "Id()", first.Id(), "main")
		testutil.Equal(t, "validates", len(p.validates), 2)
		if p.validates[0] == nil || p.validates[0] == p.validates[1] {
			t.Fatal("Register() was not given distinct validates")
		}
	})
}

func Test_Validate(t *testing.T) {
	t.Run("should accept a valid struct", func(t *testing.T) {
		v := newValidator(t, newParser(t), nil)
		valid := user{Name: "ann", Inner: inner{Value: "a"}}
		testutil.Equal(t, "Validate()", v.Validate(valid), nil)
		testutil.Equal(t, "Validate()", v.Validate(&valid), nil)
	})

	t.Run("should describe the failures with the parser", func(t *testing.T) {
		v := newValidator(t, newParser(t), nil)
		testutil.Equal(t, "Validate()", v.Validate(&user{}), any([]flam.ValidationError{
			{ParamId: 1, ParamName: "Name", ErrorId: 104, ErrorMessage: "Key: 'user.Name' Error:Field validation for 'Name' failed on the 'required' tag"},
			{ParamId: 0, ParamName: "Inner", ErrorId: 104, ErrorMessage: "Key: 'user.Inner' Error:Field validation for 'Inner' failed on the 'required' tag"},
		}))
	})

	t.Run("should convert the failures with the converter", func(t *testing.T) {
		v := newValidator(t, newParser(t), converter{})
		testutil.Equal(t, "Validate()", v.Validate(user{Inner: inner{Value: "a"}}), any([]string{"Name"}))
		testutil.Equal(t, "Validate()", v.Validate(user{Name: "ann", Inner: inner{Value: "a"}}), nil)
	})

	t.Run("should take a nil converter as none", func(t *testing.T) {
		v := newValidator(t, newParser(t), (*converter)(nil))
		got := v.Validate(user{Inner: inner{Value: "a"}})
		if _, ok := got.([]flam.ValidationError); !ok {
			t.Fatalf("Validate() = %#v, want []flam.ValidationError", got)
		}
	})

	t.Run("should fail on a value that is not a struct", func(t *testing.T) {
		v := newValidator(t, newParser(t), converter{})
		for value, typ := range map[any]string{7: "int", "a": "string", (*user)(nil): "*validators.user"} {
			err, ok := v.Validate(value).(error)
			if !ok {
				t.Fatalf("Validate(%v) is not an error", value)
			}
			cause := &govalidator.InvalidValidationError{Type: reflect.TypeOf(value)}
			testutil.Error(t, err, validator.ErrInvalidValue, "unable to validate: invalid validation value: "+cause.Error(),
				map[string]any{validator.ErrorFieldID: "main", validator.ErrorFieldType: typ})
		}
		err, ok := v.Validate(nil).(error)
		if !ok || !errors.Is(err, validator.ErrInvalidValue) {
			t.Fatalf("Validate(nil) = %v, want it to wrap ErrInvalidValue", err)
		}
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		v := newValidator(t, newParser(t), converter{})
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				testutil.Equal(t, "Validate()", v.Validate(&user{}), any([]string{"Name", "Inner"}))
			})
		}
		wg.Wait()
	})
}

func Test_Creator(t *testing.T) {
	invalid := user{Inner: inner{Value: "a"}}

	t.Run("should create the default validator", func(t *testing.T) {
		v, err := newDriverValidator(t, standard.Id, nil, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", v.Id(), standard.Id)
		got, ok := v.Validate(invalid).([]flam.ValidationError)
		if !ok || len(got) != 1 || got[0].ParamId != 1 {
			t.Fatalf("Validate() = %#v, want the error of Name", got)
		}
	})

	t.Run("should take the default converter", func(t *testing.T) {
		v, err := newDriverValidator(t, standard.Id, map[string]any{validator.DefaultConverterConfigPath: "names"}, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "Validate()", v.Validate(invalid), any([]string{"Name"}))
	})

	t.Run("should take the fields of the configuration", func(t *testing.T) {
		v, err := newDriverValidator(t, "other", map[string]any{
			validator.DefaultParserConfigPath:    "unknown",
			validator.DefaultConverterConfigPath: "unknown",
			validator.ParserConfigPath + ".mine": container.Bag{factory.ConfigFieldDriver: parserstandard.Driver},
			validator.ValidatorConfigPath + ".other": container.Bag{
				factory.ConfigFieldDriver:      standard.Driver,
				validator.ConfigFieldParser:    "mine",
				validator.ConfigFieldConverter: "names",
			},
		}, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "Validate()", v.Validate(invalid), any([]string{"Name"}))
	})

	t.Run("should have no converter with an empty converter", func(t *testing.T) {
		v, err := newDriverValidator(t, "other", map[string]any{
			validator.DefaultConverterConfigPath: "names",
			validator.ValidatorConfigPath + ".other": container.Bag{
				factory.ConfigFieldDriver:      standard.Driver,
				validator.ConfigFieldConverter: "",
			},
		}, false)
		testutil.NoError(t, err)
		if _, ok := v.Validate(invalid).([]flam.ValidationError); !ok {
			t.Fatal("Validate() converted the failures")
		}
	})

	for name, tc := range map[string]struct {
		values        map[string]any
		withoutParser bool
		field         string
	}{
		"a missing parser": {
			nil, true, validator.ConfigFieldParser,
		},
		"an empty parser": {
			map[string]any{validator.ValidatorConfigPath + ".standard." + validator.ConfigFieldParser: ""}, false, validator.ConfigFieldParser,
		},
		"a parser that is not a string": {
			map[string]any{validator.ValidatorConfigPath + ".standard." + validator.ConfigFieldParser: 7}, false, validator.ConfigFieldParser,
		},
		"a default parser that is not a string": {
			map[string]any{validator.DefaultParserConfigPath: 7}, false, validator.ConfigFieldParser,
		},
		"a converter that is not a string": {
			map[string]any{validator.ValidatorConfigPath + ".standard." + validator.ConfigFieldConverter: 7}, false, validator.ConfigFieldConverter,
		},
		"a default converter that is not a string": {
			map[string]any{validator.DefaultConverterConfigPath: true}, false, validator.ConfigFieldConverter,
		},
	} {
		t.Run("should fail on "+name, func(t *testing.T) {
			_, err := newDriverValidator(t, standard.Id, tc.values, tc.withoutParser)
			creatorError(t, err, standard.ErrInvalidField, "unable to create standard validator: invalid standard validator field",
				map[string]any{standard.ErrorFieldID: standard.Id, standard.ErrorFieldField: tc.field})
		})
	}

	t.Run("should fail on an unknown parser", func(t *testing.T) {
		_, err := newDriverValidator(t, standard.Id, map[string]any{validator.DefaultParserConfigPath: "unknown"}, false)
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want it to wrap factory.ErrUnknownID", err)
		}
		creatorError(t, err, standard.ErrParser, errors.Unwrap(err).Error(),
			map[string]any{standard.ErrorFieldID: standard.Id, standard.ErrorFieldParser: "unknown"})
	})

	t.Run("should fail on an unknown converter", func(t *testing.T) {
		_, err := newDriverValidator(t, standard.Id, map[string]any{validator.DefaultConverterConfigPath: "unknown"}, false)
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want it to wrap factory.ErrUnknownID", err)
		}
		creatorError(t, err, standard.ErrConverter, errors.Unwrap(err).Error(),
			map[string]any{standard.ErrorFieldID: standard.Id, standard.ErrorFieldConverter: "unknown"})
	})
}
