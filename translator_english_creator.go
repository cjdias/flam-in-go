package flam

import (
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
)

type englishTranslatorCreator struct{}

func newEnglishTranslatorCreator() TranslatorCreator {
	return &englishTranslatorCreator{}
}

func (englishTranslatorCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == TranslatorDriverEnglish
}

func (englishTranslatorCreator) Create(
	_ Bag,
) (Translator, error) {
	lang := en.New()
	translator, found := ut.New(lang, lang).GetTranslator("en")
	if !found {
		return nil, newErrLanguageNotFound("en")
	}

	return Translator(
		translator), nil
}
