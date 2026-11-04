package envelope

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
)

// NewConverter returns the [flam.IValidationErrorConverter] of the id
// converting the failures with [Convert] into envelopes of errors of the
// status. It fails with ErrInvalidStatus when the status is out of the
// 400 to 599 range of the error statuses.
func NewConverter(
	id string,
	status int,
) (flam.IValidationErrorConverter, error) {
	if status < 400 || status > 599 {
		return nil, newErrInvalidStatus(converterNewMessage, id, status)
	}
	return &converter{
		id:     id,
		status: status,
	}, nil
}

// Convert returns the envelope holding one error of the status per
// failure, in order, identified by the ParamId and the ErrorId of the
// failure, those of 0 being left out, with the ErrorMessage of the
// failure, unless empty, at
// [github.com/cjdias/flam-in-go/pkg/flam/envelope.ContextFieldMessage],
// and its ParamName at [ContextFieldParam], in its context. No failure
// gives an envelope of no error.
func Convert(
	status int,
	errs []flam.ValidationError,
) envelope.Envelope {
	env := envelope.NewEnvelope()
	for _, e := range errs {
		ctx := map[string]any{ContextFieldParam: e.ParamName}
		if e.ErrorMessage != "" {
			ctx[envelope.ContextFieldMessage] = e.ErrorMessage
		}
		envErr := envelope.NewError(status, nil, ctx).WithParamId(e.ParamId)
		if e.ErrorId != 0 {
			envErr = envErr.WithErrorId(e.ErrorId)
		}
		env = env.WithError(envErr)
	}
	return env
}

// converter is the [flam.IValidationErrorConverter] returned by
// [NewConverter].
type converter struct {
	id     string
	status int
}

// compile time check that *converter implements IValidationErrorConverter
var _ flam.IValidationErrorConverter = (*converter)(nil)

func (c *converter) Id() string {
	return c.id
}

// Convert returns the [envelope.Envelope] of the failures, as [Convert]
// returns it with the status of the converter.
func (c *converter) Convert(
	errs []flam.ValidationError,
) any {
	return Convert(c.status, errs)
}

// converterNewMessage is the message of the errors returned by
// NewConverter.
const converterNewMessage = "unable to create envelope validation converter"
