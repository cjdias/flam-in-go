package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	envelopeconverter "github.com/cjdias/flam-in-go/pkg/flam/validator/converters/envelope"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// validateStub is a validator answering its result, recording the values
// it validates.
type validateStub struct {
	result any
	values []any
}

func (v *validateStub) Id() string { return "stub" }

func (v *validateStub) Validate(value any) any {
	v.values = append(v.values, value)
	return v.result
}

// validateBody is the body of the requests of the validation tests.
type validateBody struct {
	Name string `json:"name"`
}

// validateFailures are the failures of a value of no name.
var validateFailures = []flam.ValidationError{
	{ParamId: 1, ParamName: "Name", ErrorId: 104, ErrorMessage: "Name is a required field"},
}

func Test_Validate_Declarations(t *testing.T) {
	testutil.Equal(t, "ContextFieldFailures", rest.ContextFieldFailures, "failures")
	for name, err := range map[string]error{
		"ErrValidation":   rest.ErrValidation,
		"ErrNilValidator": rest.ErrNilValidator,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Validate(t *testing.T) {
	t.Run("should accept a valid value", func(t *testing.T) {
		v := &validateStub{}
		value := &validateBody{Name: "ann"}
		env, ok := rest.Validate(v, value)
		testutil.Equal(t, "ok", ok, true)
		testutil.Equal(t, "env", env, envelope.NewEnvelope())
		testutil.Equal(t, "values", v.values, []any{value})
	})

	t.Run("should fail with a nil validator", func(t *testing.T) {
		for _, v := range []flam.IValidator{nil, (*validateStub)(nil)} {
			env, ok := rest.Validate(v, &validateBody{})
			testutil.Equal(t, "ok", ok, false)
			testutil.Equal(t, "env", env, envelope.NewEnvelope().WithNewError(http.StatusInternalServerError, rest.ErrNilValidator))
		}
	})

	failures := envelopeconverter.Convert(http.StatusUnprocessableEntity, validateFailures)
	invalidValue := flam.NewWrappedError(fmt.Errorf("%w: not a struct", validator.ErrInvalidValue), "unable to validate")
	for name, tc := range map[string]struct {
		result any
		want   envelope.Envelope
	}{
		"an envelope": {
			failures,
			failures,
		},
		"an envelope of no error": {
			envelope.NewEnvelope("data"),
			envelope.NewEnvelope("data").WithNewError(http.StatusBadRequest, rest.ErrValidation),
		},
		"failures": {
			validateFailures,
			envelopeconverter.Convert(http.StatusBadRequest, validateFailures),
		},
		"no failure": {
			[]flam.ValidationError{},
			envelope.NewEnvelope().WithNewError(http.StatusBadRequest, rest.ErrValidation),
		},
		"an error of an invalid value": {
			invalidValue,
			envelope.NewEnvelope().WithNewError(http.StatusInternalServerError, invalidValue),
		},
		"another error": {
			errBoom,
			envelope.NewEnvelope().WithNewError(http.StatusBadRequest, errBoom),
		},
		"another value": {
			[]string{"Name"},
			envelope.NewEnvelope().WithNewError(http.StatusBadRequest, rest.ErrValidation,
				map[string]any{rest.ContextFieldFailures: []string{"Name"}}),
		},
	} {
		t.Run("should answer the envelope of "+name, func(t *testing.T) {
			env, ok := rest.Validate(&validateStub{result: tc.result}, &validateBody{})
			testutil.Equal(t, "ok", ok, false)
			testutil.Equal(t, "env", env, tc.want)
		})
	}

	t.Run("should be written as the problem of its first error", func(t *testing.T) {
		env, _ := rest.Validate(&validateStub{result: validateFailures}, &validateBody{})
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		w := httptest.NewRecorder()
		testutil.NoError(t, rest.WriteEnvelope(w, req, env))
		testutil.Equal(t, "status", w.Code, http.StatusBadRequest)
		testutil.Equal(t, "body", strings.TrimSpace(w.Body.String()),
			`{"detail":"Name is a required field","id":"p:1.c:104","param":"Name","status":400,`+
				`"title":"Bad Request","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/400"}`)
	})
}

func Test_DecodeJSONAndValidate(t *testing.T) {
	t.Run("should decode and validate the body", func(t *testing.T) {
		v := &validateStub{}
		var body validateBody
		env, ok := rest.DecodeJSONAndValidate(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ann"}`)), v, &body)
		testutil.Equal(t, "ok", ok, true)
		testutil.Equal(t, "env", env, envelope.NewEnvelope())
		testutil.Equal(t, "body", body, validateBody{Name: "ann"})
		testutil.Equal(t, "values", v.values, []any{&body})
	})

	t.Run("should answer the envelope of the validation", func(t *testing.T) {
		var body validateBody
		env, ok := rest.DecodeJSONAndValidate(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)),
			&validateStub{result: validateFailures}, &body)
		testutil.Equal(t, "ok", ok, false)
		testutil.Equal(t, "env", env, envelopeconverter.Convert(http.StatusBadRequest, validateFailures))
	})

	t.Run("should answer the decoding error without validating", func(t *testing.T) {
		v := &validateStub{}
		var body validateBody
		env, ok := rest.DecodeJSONAndValidate(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`)), v, &body)
		testutil.Equal(t, "ok", ok, false)
		testutil.Equal(t, "errors", len(env.Errors), 1)
		testutil.Equal(t, "status", env.Errors[0].Status, http.StatusBadRequest)
		if !strings.HasPrefix(env.Errors[0].Error(), "unable to decode rest request: ") ||
			env.Errors[0].Context[rest.ErrorFieldSource] != rest.SourceBody {
			t.Fatalf("Errors = %#v, want the decoding error of the body", env.Errors)
		}
		testutil.Equal(t, "values", len(v.values), 0)
	})
}
