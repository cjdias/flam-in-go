package validator

import (
	"errors"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/validators"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newApp returns a testutil.App with the config and validator bundles.
func newApp(t *testing.T) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(validator.NewBundle()))
	return a
}

var errBoom = errors.New("boom")

// failingParser is a parser failing to close.
type failingParser struct{ flam.IValidationParser }

func (failingParser) Id() string   { return "bad" }
func (failingParser) Close() error { return errBoom }

// failingConverter is a converter failing to close.
type failingConverter struct{ flam.IValidationErrorConverter }

func (failingConverter) Id() string   { return "bad" }
func (failingConverter) Close() error { return errBoom }

// failingValidator is a validator failing to close.
type failingValidator struct{ flam.IValidator }

func (failingValidator) Id() string   { return "bad" }
func (failingValidator) Close() error { return errBoom }

// converter is the converter of the application, keeping the names of
// the fields.
type converter struct{}

func (converter) Id() string { return "names" }

func (converter) Convert(errs []flam.ValidationError) any {
	names := make([]string, len(errs))
	for i, e := range errs {
		names[i] = e.ParamName
	}
	return names
}

// converterCreator creates the converter of the application.
type converterCreator struct{}

func (converterCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "names"
}

func (converterCreator) Create(flam.IBag) (flam.IValidationErrorConverter, error) {
	return converter{}, nil
}

type user struct {
	Name  string `validate:"required" paramId:"1"`
	Email string `validate:"required,email" paramId:"2"`
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ParserConfigPath", validator.ParserConfigPath, "flam.validator.parsers")
	testutil.Equal(t, "ParserCreatorGroup", validator.ParserCreatorGroup, "flam.validator.parsers.creator")
	testutil.Equal(t, "ConverterConfigPath", validator.ConverterConfigPath, "flam.validator.converters")
	testutil.Equal(t, "ConverterCreatorGroup", validator.ConverterCreatorGroup, "flam.validator.converters.creator")
	testutil.Equal(t, "ValidatorConfigPath", validator.ValidatorConfigPath, "flam.validator.validators")
	testutil.Equal(t, "ValidatorCreatorGroup", validator.ValidatorCreatorGroup, "flam.validator.validators.creator")
	testutil.Equal(t, "ConfigFieldParser", validator.ConfigFieldParser, "parser")
	testutil.Equal(t, "ConfigFieldConverter", validator.ConfigFieldConverter, "converter")
	testutil.Equal(t, "DefaultParserConfigPath", validator.DefaultParserConfigPath, "flam.validator.defaults.validators.parser")
	testutil.Equal(t, "DefaultConverterConfigPath", validator.DefaultConverterConfigPath, "flam.validator.defaults.validators.converter")
	testutil.Equal(t, "ErrorFieldID", validator.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldType", validator.ErrorFieldType, "type")
	if validator.ErrInvalidValue == nil {
		t.Error("ErrInvalidValue = nil")
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories", func(t *testing.T) {
		b := validator.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator")

		a := newApp(t)
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-4:], []string{"bundle:flam.validator", "provider/0", "provider/0", "provider/0"})
		testutil.NoError(t, a.Invoke(func(
			p flam.IValidationParserFactory, c flam.IValidationErrorConverterFactory, v flam.IValidatorFactory,
		) {
			testutil.Equal(t, "parsers", p.Available(), []string{})
			testutil.Equal(t, "converters", c.Available(), []string{})
			testutil.Equal(t, "validators", v.Available(), []string{})
		}))
	})

	t.Run("should close every factory, joining their errors", func(t *testing.T) {
		a := newApp(t)
		var p flam.IValidationParserFactory
		var c flam.IValidationErrorConverterFactory
		var v flam.IValidatorFactory
		testutil.NoError(t, a.Invoke(func(
			pf flam.IValidationParserFactory, cf flam.IValidationErrorConverterFactory, vf flam.IValidatorFactory,
		) {
			p, c, v = pf, cf, vf
		}))
		testutil.NoError(t, p.Store("bad", failingParser{}))
		testutil.NoError(t, c.Store("bad", failingConverter{}))
		testutil.NoError(t, v.Store("bad", failingValidator{}))

		err := validator.NewBundle().(flam.IClosableBundle).Close(a)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
		testutil.Equal(t, "errors", len(err.(interface{ Unwrap() []error }).Unwrap()), 3)
		for name, f := range map[string]interface{ Has(string) bool }{"parsers": p, "converters": c, "validators": v} {
			testutil.Equal(t, name+" closed", f.Has("bad"), false)
		}
		testutil.NoError(t, validator.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to close without the factory dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(validator.NewBundle()))
		if err := validator.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}

func Test_Application(t *testing.T) {
	values := container.Bag{}
	testutil.NoError(t, values.Set(standard.DefaultTranslatorConfigPath, "pt"))
	testutil.NoError(t, values.Set(validator.ConverterConfigPath+".names", container.Bag{factory.ConfigFieldDriver: "names"}))
	testutil.NoError(t, values.Set(validator.ValidatorConfigPath+".api", container.Bag{
		factory.ConfigFieldDriver:      "flam.validator.validators.driver.standard",
		validator.ConfigFieldConverter: "names",
	}))
	a := app.New(values)
	for _, b := range []flam.IBundle{
		translator.NewBundle(), translators.NewBundle(),
		validator.NewBundle(), parsers.NewBundle(), validators.NewBundle(),
	} {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Provide(func() flam.IValidationErrorConverterCreator { return converterCreator{} },
		dig.Group(validator.ConverterCreatorGroup)))
	testutil.NoError(t, a.Boot())

	testutil.NoError(t, a.Invoke(func(f flam.IValidatorFactory) error {
		v, err := f.Get("standard")
		if err != nil {
			return err
		}
		testutil.Equal(t, "Validate()", v.Validate(&user{Name: "ann", Email: "ann"}), any([]flam.ValidationError{
			{ParamId: 2, ParamName: "Email", ErrorId: 63, ErrorMessage: "Email deve ser um endereço de e-mail válido"},
		}))
		testutil.Equal(t, "Validate()", v.Validate(user{Name: "ann", Email: "ann@example.com"}), nil)

		api, err := f.Get("api")
		if err != nil {
			return err
		}
		testutil.Equal(t, "Validate()", api.Validate(&user{}), any([]string{"Name", "Email"}))
		return nil
	}))
	testutil.NoError(t, a.Close())
}
