package flam

import (
	ut "github.com/go-playground/universal-translator"
)

// ITranslator translates the keys of its texts into the language of its
// locale, with the [ut.Translator] of go-playground, which also formats
// the numbers, dates and currencies of the locale. Its texts take the {0},
// {1}... parameters, and its plural texts, chosen by the cardinal, ordinal
// or range plural rule of a number, the {0} and {1} ones. It may be used
// from several goroutines, texts being added while others are translated.
// It is held by an [ITranslatorFactory].
type ITranslator interface {
	IResource

	ut.Translator
}

// ITranslatorCreator creates the translators of a driver, for the
// [ITranslatorFactory].
type ITranslatorCreator = IResourceCreator[ITranslator]

// ITranslatorFactory creates and holds the translators, one per id.
type ITranslatorFactory = IFactory[ITranslator]
