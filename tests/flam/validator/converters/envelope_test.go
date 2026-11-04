package converters

import (
	"errors"
	"net/http"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamenvelope "github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/converters"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/converters/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers"
	parserstandard "github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/validators"
	validatorstandard "github.com/cjdias/flam-in-go/pkg/flam/validator/validators/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

type user struct {
	Name  string `validate:"required" paramId:"1"`
	Email string `validate:"required,email" paramId:"2"`
}

// failures are the failures of a user of no name and an invalid email,
// one of no message and no code.
var failures = []flam.ValidationError{
	{ParamId: 1, ParamName: "Name", ErrorId: 104, ErrorMessage: "Name is a required field"},
	{ParamId: 0, ParamName: "Email", ErrorId: 0, ErrorMessage: ""},
}

// newApp returns a testutil.App with the config, validator and converter
// bundles, along with the bundles of the parsers and the validators when
// withValidators, the configuration holding the defaults of the
// configurable bundles and the values.
func newApp(t *testing.T, values map[string]any, withValidators bool) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	registered := []flam.IBundle{config.NewBundle(), validator.NewBundle(), converters.NewBundle()}
	if withValidators {
		registered = append(registered, parsers.NewBundle(), validators.NewBundle())
	}
	for _, b := range registered {
		testutil.NoError(t, a.RegisterBundle(b))
	}

	bag := container.Bag{}
	testutil.NoError(t, envelope.NewBundle().(flam.IConfigurableBundle).Config(bag))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// newDriverConverter returns the converter of the id of an app whose
// configuration holds the values.
func newDriverConverter(t *testing.T, id string, values map[string]any) (flam.IValidationErrorConverter, error) {
	t.Helper()
	var c flam.IValidationErrorConverter
	var err error
	testutil.NoError(t, newApp(t, values, false).Invoke(func(f flam.IValidationErrorConverterFactory) {
		c, err = f.Get(id)
	}))
	return c, err
}

// statuses returns the statuses of the errors of the envelope.
func statuses(env flamenvelope.Envelope) []int {
	got := make([]int, len(env.Errors))
	for i, e := range env.Errors {
		got[i] = e.Status
	}
	return got
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                      {envelope.Id, "envelope"},
		"Driver":                  {envelope.Driver, "flam.validator.converters.driver.envelope"},
		"ConfigFieldStatus":       {envelope.ConfigFieldStatus, "status"},
		"DefaultStatusConfigPath": {envelope.DefaultStatusConfigPath, "flam.validator.defaults.converters.envelope.status"},
		"ContextFieldParam":       {envelope.ContextFieldParam, "param"},
		"ErrorFieldID":            {envelope.ErrorFieldID, "id"},
		"ErrorFieldField":         {envelope.ErrorFieldField, "field"},
		"ErrorFieldStatus":        {envelope.ErrorFieldStatus, "status"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrInvalidField":  envelope.ErrInvalidField,
		"ErrInvalidStatus": envelope.ErrInvalidStatus,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundles(t *testing.T) {
	t.Run("should configure the default converter", func(t *testing.T) {
		b := envelope.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator.converters.envelope")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "converters", bag.Get(validator.ConverterConfigPath), container.Bag{
			envelope.Id: container.Bag{factory.ConfigFieldDriver: envelope.Driver},
		})
		testutil.Equal(t, "status", bag.Get(envelope.DefaultStatusConfigPath), 400)
		testutil.Equal(t, "default converter", bag.Get(validator.DefaultConverterConfigPath), nil)

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.validator.converters.envelope", "provider/1"})
	})

	t.Run("should fail to configure a bag that cannot hold the paths", func(t *testing.T) {
		b := envelope.NewBundle().(flam.IConfigurableBundle)
		if err := b.Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail on the converter path")
		}
		bag := container.Bag{"flam": container.Bag{"validator": container.Bag{"defaults": "scalar"}}}
		if err := b.Config(bag); err == nil {
			t.Fatal("Config() did not fail on the status path")
		}
	})

	t.Run("should register every converter driver", func(t *testing.T) {
		b := converters.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.validator.converters")
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.validator.converters", "bundle:flam.validator.converters.envelope", "provider/1",
		})
	})
}

func Test_NewConverter(t *testing.T) {
	t.Run("should fail on a status out of the error range", func(t *testing.T) {
		for _, status := range []int{0, 200, 399, 600} {
			c, err := envelope.NewConverter("main", status)
			testutil.Error(t, err, envelope.ErrInvalidStatus,
				"unable to create envelope validation converter: invalid envelope validation converter status",
				map[string]any{envelope.ErrorFieldID: "main", envelope.ErrorFieldStatus: status})
			if c != nil {
				t.Fatalf("NewConverter(%d) = %v, want nil", status, c)
			}
		}
	})

	t.Run("should convert with the status", func(t *testing.T) {
		for _, status := range []int{400, 422, 599} {
			c, err := envelope.NewConverter("main", status)
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", c.Id(), "main")
			testutil.Equal(t, "Convert()", c.Convert(failures), any(envelope.Convert(status, failures)))
		}
	})
}

func Test_Convert(t *testing.T) {
	t.Run("should hold one error per failure", func(t *testing.T) {
		env := envelope.Convert(http.StatusUnprocessableEntity, failures)
		testutil.Equal(t, "Data", env.Data, nil)
		testutil.Equal(t, "Filter", env.Filter, nil)
		testutil.Equal(t, "Errors", env.Errors, []flamenvelope.Error{
			flamenvelope.NewError(http.StatusUnprocessableEntity, nil, map[string]any{
				envelope.ContextFieldParam:       "Name",
				flamenvelope.ContextFieldMessage: "Name is a required field",
			}).WithParamId(1).WithErrorId(104),
			flamenvelope.NewError(http.StatusUnprocessableEntity, nil, map[string]any{
				envelope.ContextFieldParam: "Email",
			}).WithParamId(0),
		})
		testutil.Equal(t, "Id", env.Errors[0].Id, "p:1.c:104")
		testutil.Equal(t, "Id", env.Errors[1].Id, "")
		testutil.Equal(t, "Error()", env.Errors[0].Error(), "Name is a required field")
		testutil.Equal(t, "Error()", env.Errors[1].Error(), "Unprocessable Entity")
	})

	t.Run("should be identified by the service and endpoint too", func(t *testing.T) {
		env := envelope.Convert(http.StatusBadRequest, failures[:1]).WithServiceId(3).WithEndpointId(4)
		testutil.Equal(t, "Id", env.Errors[0].Id, "s:3.e:4.p:1.c:104")
	})

	t.Run("should give an envelope of no error for no failure", func(t *testing.T) {
		testutil.Equal(t, "Convert(nil)", envelope.Convert(http.StatusBadRequest, nil), flamenvelope.NewEnvelope())
		testutil.Equal(t, "Convert([])", envelope.Convert(http.StatusBadRequest, []flam.ValidationError{}),
			flamenvelope.NewEnvelope())
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should create the default converter", func(t *testing.T) {
		c, err := newDriverConverter(t, envelope.Id, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", c.Id(), envelope.Id)
		testutil.Equal(t, "statuses", statuses(c.Convert(failures).(flamenvelope.Envelope)), []int{400, 400})
	})

	t.Run("should take the default status", func(t *testing.T) {
		c, err := newDriverConverter(t, envelope.Id, map[string]any{envelope.DefaultStatusConfigPath: 422})
		testutil.NoError(t, err)
		testutil.Equal(t, "statuses", statuses(c.Convert(failures).(flamenvelope.Envelope)), []int{422, 422})
	})

	t.Run("should take the built-in status without a default", func(t *testing.T) {
		a := testutil.NewApp()
		for _, b := range []flam.IBundle{config.NewBundle(), validator.NewBundle(), converters.NewBundle()} {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(validator.ConverterConfigPath+".other", container.Bag{factory.ConfigFieldDriver: envelope.Driver}))
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
		testutil.NoError(t, a.Invoke(func(f flam.IValidationErrorConverterFactory) {
			c, err := f.Get("other")
			testutil.NoError(t, err)
			testutil.Equal(t, "statuses", statuses(c.Convert(failures).(flamenvelope.Envelope)), []int{400, 400})
		}))
	})

	t.Run("should take the status of the configuration", func(t *testing.T) {
		c, err := newDriverConverter(t, "other", map[string]any{
			envelope.DefaultStatusConfigPath: 500,
			validator.ConverterConfigPath + ".other": container.Bag{
				factory.ConfigFieldDriver:  envelope.Driver,
				envelope.ConfigFieldStatus: 409,
			},
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", c.Id(), "other")
		testutil.Equal(t, "statuses", statuses(c.Convert(failures).(flamenvelope.Envelope)), []int{409, 409})
	})

	for name, values := range map[string]map[string]any{
		"a status that is not an int": {
			validator.ConverterConfigPath + ".envelope." + envelope.ConfigFieldStatus: "400",
		},
		"a default status that is not an int": {
			envelope.DefaultStatusConfigPath: 400.0,
		},
	} {
		t.Run("should fail on "+name, func(t *testing.T) {
			_, err := newDriverConverter(t, envelope.Id, values)
			var e *flam.Error
			if !errors.Is(err, envelope.ErrInvalidField) || !errors.As(errors.Unwrap(err), &e) {
				t.Fatalf("Get() = %v, want it to wrap ErrInvalidField", err)
			}
			testutil.Equal(t, "Error()", e.Error(), "unable to create envelope validation converter: invalid envelope validation converter field")
			testutil.Context(t, e, map[string]any{envelope.ErrorFieldID: envelope.Id, envelope.ErrorFieldField: envelope.ConfigFieldStatus})
		})
	}

	t.Run("should fail on a status out of the error range", func(t *testing.T) {
		_, err := newDriverConverter(t, envelope.Id, map[string]any{envelope.DefaultStatusConfigPath: 302})
		var e *flam.Error
		if !errors.Is(err, envelope.ErrInvalidStatus) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want it to wrap ErrInvalidStatus", err)
		}
		testutil.Context(t, e, map[string]any{envelope.ErrorFieldID: envelope.Id, envelope.ErrorFieldStatus: 302})
	})

	t.Run("should not accept the configurations of another driver", func(t *testing.T) {
		_, err := newDriverConverter(t, "other", map[string]any{
			validator.ConverterConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "unknown"},
		})
		if err == nil {
			t.Fatal("Get() did not fail")
		}
	})
}

func Test_Validator(t *testing.T) {
	t.Run("should answer the envelopes of the failures as the default converter", func(t *testing.T) {
		a := newApp(t, map[string]any{validator.DefaultConverterConfigPath: envelope.Id}, true)
		bag := container.Bag{}
		for _, b := range []flam.IBundle{parserstandard.NewBundle(), validatorstandard.NewBundle()} {
			testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		}
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

		testutil.NoError(t, a.Invoke(func(f flam.IValidatorFactory) {
			v, err := f.Get(validatorstandard.Id)
			testutil.NoError(t, err)
			testutil.Equal(t, "valid", v.Validate(&user{Name: "ann", Email: "ann@example.com"}), nil)
			env, ok := v.Validate(&user{Email: "bad"}).(flamenvelope.Envelope)
			if !ok || len(env.Errors) != 2 {
				t.Fatalf("Validate() = %#v, want an envelope of 2 errors", env)
			}
			testutil.Equal(t, "Id", env.Errors[0].Id, "p:1.c:104")
			testutil.Equal(t, "param", env.Errors[0].Context[envelope.ContextFieldParam], "Name")
			testutil.Equal(t, "Id", env.Errors[1].Id, "p:2.c:63")
			testutil.Equal(t, "param", env.Errors[1].Context[envelope.ContextFieldParam], "Email")
			testutil.Equal(t, "statuses", statuses(env), []int{400, 400})
		}))
	})
}
