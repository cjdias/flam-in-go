package rest

import (
	"errors"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	envelopeconverter "github.com/cjdias/flam-in-go/pkg/flam/validator/converters/envelope"
)

// ContextFieldFailures is the field of the context of the envelope error
// returned by [Validate] holding the result of a validator that is
// neither an envelope, nor failures, nor an error.
const ContextFieldFailures = "failures"

var (
	// ErrValidation is the error of the envelope returned by [Validate]
	// when the result of the validator holds no error of its own, such as
	// an envelope of no error, or a value of another type.
	ErrValidation = errors.New("rest request failed validation")

	// ErrNilValidator is the error of the envelope returned by [Validate]
	// when the validator is nil.
	ErrNilValidator = errors.New("nil rest validator")
)

// Validate validates the value, such as a pointer to the decoded body of
// a request, with the validator, returning true when it is valid, or else
// false and the envelope to answer the request with, written by
// [WriteEnvelope]:
//
//	if env, ok := rest.Validate(r.validator, &body); !ok {
//		_ = rest.WriteEnvelope(w, req, env)
//		return
//	}
//
// The envelope depends on the result of the validator:
//   - an [envelope.Envelope], as answered by a converter of
//     [github.com/cjdias/flam-in-go/pkg/flam/validator/converters/envelope],
//     is returned as it is, with an error of status 400 wrapping
//     ErrValidation when it holds none;
//   - the [flam.ValidationError] values of a validator without converter
//     are converted, as the converter of status 400 converts them;
//   - an error wrapping
//     [github.com/cjdias/flam-in-go/pkg/flam/validator.ErrInvalidValue],
//     the value being no struct, is an error of status 500, and any other
//     error an error of status 400;
//   - any other value is an error of status 400 wrapping ErrValidation,
//     the value held by its context at [ContextFieldFailures], as it is
//     sent in the response.
//
// A nil validator gives an error of status 500 wrapping ErrNilValidator.
func Validate(
	v flam.IValidator,
	val any,
) (envelope.Envelope, bool) {
	if value.IsNil(v) {
		return envelope.NewEnvelope().WithNewError(http.StatusInternalServerError, ErrNilValidator), false
	}

	switch result := v.Validate(val).(type) {
	case nil:
		return envelope.NewEnvelope(), true
	case envelope.Envelope:
		if len(result.Errors) == 0 {
			result = result.WithNewError(http.StatusBadRequest, ErrValidation)
		}
		return result, false
	case []flam.ValidationError:
		env := envelopeconverter.Convert(http.StatusBadRequest, result)
		if len(env.Errors) == 0 {
			env = env.WithNewError(http.StatusBadRequest, ErrValidation)
		}
		return env, false
	case error:
		status := http.StatusBadRequest
		if errors.Is(result, validator.ErrInvalidValue) {
			status = http.StatusInternalServerError
		}
		return envelope.NewEnvelope().WithNewError(status, result), false
	default:
		return envelope.NewEnvelope().WithNewError(http.StatusBadRequest, ErrValidation,
			map[string]any{ContextFieldFailures: result}), false
	}
}

// DecodeJSONAndValidate decodes the JSON body of the request into the
// value, a pointer, with [DecodeJSON], then validates it with [Validate],
// returning true when both succeed, or else false and the envelope to
// answer the request with: an error of status 400 wrapping ErrDecode when
// the body cannot be decoded, or the envelope of [Validate].
//
//	var body createUser
//	if env, ok := rest.DecodeJSONAndValidate(req, r.validator, &body); !ok {
//		_ = rest.WriteEnvelope(w, req, env)
//		return
//	}
func DecodeJSONAndValidate(
	req *http.Request,
	v flam.IValidator,
	val any,
) (envelope.Envelope, bool) {
	if err := DecodeJSON(req, val); err != nil {
		return envelope.NewEnvelope().WithNewError(http.StatusBadRequest, err), false
	}
	return Validate(v, val)
}
