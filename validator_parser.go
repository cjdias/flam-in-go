package flam

import "github.com/go-playground/validator/v10"

type ValidatorParser interface {
	AddTagCode(tag string, code int)
	Parse(value any, errs validator.ValidationErrors) []ValidationError
}
