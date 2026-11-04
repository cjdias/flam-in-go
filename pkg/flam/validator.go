package flam

import (
	"github.com/go-playground/validator/v10"
)

// ValidationError describes a field of a value failing a rule of its
// validate struct tag, as reported by an [IValidationParser].
type ValidationError struct {
	// ParamId is the id of the field, read from the struct tag of the
	// field named by the parser, such as `paramId:"3"`, or 0.
	ParamId int

	// ParamName is the path of the field from the validated struct, such
	// as "Name", "Address.Street" or "Items[0]".
	ParamName string

	// ErrorId is the code of the failed rule, such as "required", or 0
	// when the parser holds no code for it.
	ErrorId int

	// ErrorMessage is the message of the failure, in the language of the
	// translator of the parser, if any.
	ErrorMessage string
}

// IValidationParser turns the errors of a go-playground
// [validator.Validate] into [ValidationError] values. It may be used from
// several goroutines. It is held by an [IValidationParserFactory].
type IValidationParser interface {
	IResource

	// Register registers on validate the translations of the messages of
	// its errors, before validate is used, so that Parse translates them.
	Register(validate *validator.Validate) error

	// AddTagCode sets the code of a rule, given as the ErrorId of its
	// errors, such as the code of a custom rule.
	AddTagCode(tag string, code int)

	// Parse returns the description of each of the errors reported by a
	// [validator.Validate] for the value.
	Parse(value any, errs validator.ValidationErrors) []ValidationError
}

// IValidationParserCreator creates the validation parsers of a driver,
// for the [IValidationParserFactory].
type IValidationParserCreator = IResourceCreator[IValidationParser]

// IValidationParserFactory creates and holds the validation parsers, one
// per id.
type IValidationParserFactory = IFactory[IValidationParser]

// IValidationErrorConverter converts the [ValidationError] values of a
// value failing its validation into the result of an [IValidator], such
// as the body of an error response. It is held by an
// [IValidationErrorConverterFactory].
type IValidationErrorConverter interface {
	IResource

	Convert(errors []ValidationError) any
}

// IValidationErrorConverterCreator creates the validation error
// converters of a driver, for the [IValidationErrorConverterFactory].
type IValidationErrorConverterCreator = IResourceCreator[IValidationErrorConverter]

// IValidationErrorConverterFactory creates and holds the validation error
// converters, one per id.
type IValidationErrorConverterFactory = IFactory[IValidationErrorConverter]

// IValidator validates the values against the rules of their validate
// struct tags. It may be used from several goroutines. It is held by an
// [IValidatorFactory].
type IValidator interface {
	IResource

	// Validate returns nil when the value is valid, or else the result of
	// its failures, as described by the validator.
	Validate(value any) any
}

// IValidatorCreator creates the validators of a driver, for the
// [IValidatorFactory].
type IValidatorCreator = IResourceCreator[IValidator]

// IValidatorFactory creates and holds the validators, one per id.
type IValidatorFactory = IFactory[IValidator]
