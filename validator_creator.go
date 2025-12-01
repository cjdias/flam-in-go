package flam

type ValidatorCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (Validator, error)
}
