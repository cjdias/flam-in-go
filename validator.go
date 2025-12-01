package flam

type Validator interface {
	Validate(value any) any
}
