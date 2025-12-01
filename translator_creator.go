package flam

type TranslatorCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (Translator, error)
}
