package flam

import "github.com/go-playground/validator/v10"

type defaultValidator struct {
	validatorParser         ValidatorParser
	validatorErrorConverter ValidatorErrorConverter
	baseValidator           *validator.Validate
}

var _ Validator = (*defaultValidator)(nil)

func newDefaultValidator(
	validatorParser ValidatorParser,
	validatorErrorConverter ValidatorErrorConverter,
) Validator {
	return &defaultValidator{
		validatorParser:         validatorParser,
		validatorErrorConverter: validatorErrorConverter,
		baseValidator:           validator.New()}
}

func (defaultValidator defaultValidator) Validate(
	value any,
) any {
	errs := defaultValidator.baseValidator.Struct(value)
	if errs == nil {
		return nil
	}

	validationErrors := defaultValidator.validatorParser.Parse(value, errs.(validator.ValidationErrors))
	if defaultValidator.validatorErrorConverter == nil {
		return validationErrors
	}

	return defaultValidator.validatorErrorConverter.Convert(validationErrors)
}
