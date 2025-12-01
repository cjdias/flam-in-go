package flam

type ValidatorErrorConverterCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (ValidatorErrorConverter, error)
}
