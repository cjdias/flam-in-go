package flam

type ValidatorParserCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (ValidatorParser, error)
}
