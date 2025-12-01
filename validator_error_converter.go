package flam

type ValidatorErrorConverter interface {
	Convert(errors []ValidationError) any
}
