package flam

import (
	"io"

	"go.uber.org/dig"
)

type TranslatorFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (Translator, error)
	Store(id string, translator Translator) error
	Remove(id string) error
	RemoveAll() error
}

type translatorFactoryArgs struct {
	dig.In

	Creators      []TranslatorCreator `group:"flam.translators.creator"`
	FactoryConfig FactoryConfig
}

func newTranslatorFactory(
	args translatorFactoryArgs,
) (TranslatorFactory, error) {
	var creators []FactoryResourceCreator[Translator]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("Translator"),
		PathTranslators)
}
