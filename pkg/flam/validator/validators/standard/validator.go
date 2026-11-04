package standard

import (
	"errors"

	govalidator "github.com/go-playground/validator/v10"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewValidator returns the [flam.IValidator] of the id validating the
// values with a new go-playground validate, requiring the structs of the
// required fields to be set, on which the parser registers its messages,
// describing the failures with the parser and converting them with the
// converter, unless nil. It fails with ErrNilParser when the parser is
// nil, and with ErrRegister when the parser fails to register.
//
// Validate returns nil for a valid struct, or a pointer to one, the
// [flam.ValidationError] values of its failures, or their conversion when
// the validator has a converter, and an error wrapping
// [github.com/cjdias/flam-in-go/pkg/flam/validator.ErrInvalidValue] for a
// value that is not a struct nor a pointer to one. The validator may be
// used from several goroutines.
func NewValidator(
	id string,
	parser flam.IValidationParser,
	converter flam.IValidationErrorConverter,
) (flam.IValidator, error) {
	if value.IsNil(parser) {
		return nil, newErrNil(ErrNilParser, validatorNewMessage, id)
	}
	if value.IsNil(converter) {
		converter = nil
	}

	validate := govalidator.New(govalidator.WithRequiredStructEnabled())
	if err := parser.Register(validate); err != nil {
		return nil, newErrJoined(ErrRegister, validatorNewMessage, id, "", "", err)
	}
	return &validatorImpl{
		id:        id,
		validate:  validate,
		parser:    parser,
		converter: converter,
	}, nil
}

// validatorImpl is the [flam.IValidator] returned by [NewValidator].
type validatorImpl struct {
	id        string
	validate  *govalidator.Validate
	parser    flam.IValidationParser
	converter flam.IValidationErrorConverter
}

// compile time check that *validatorImpl implements IValidator
var _ flam.IValidator = (*validatorImpl)(nil)

func (v *validatorImpl) Id() string {
	return v.id
}

func (v *validatorImpl) Validate(
	value any,
) any {
	err := v.validate.Struct(value)
	if err == nil {
		return nil
	}

	var errs govalidator.ValidationErrors
	if !errors.As(err, &errs) {
		return newErrInvalidValue(validatorValidateMessage, v.id, value, err)
	}
	parsed := v.parser.Parse(value, errs)
	if v.converter == nil {
		return parsed
	}
	return v.converter.Convert(parsed)
}

// Messages of the errors returned by NewValidator and Validate.
const (
	validatorNewMessage      = "unable to create standard validator"
	validatorValidateMessage = "unable to validate"
)
