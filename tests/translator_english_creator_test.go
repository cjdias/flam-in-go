package tests

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_EnglishTranslatorCreator(t *testing.T) {
	t.Run("should return translation generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		utNewFuncMock := ut.New(nil)
		patches := gomonkey.ApplyFunc(ut.New, func(fallback locales.Translator, supportedLocales ...locales.Translator) *ut.UniversalTranslator {
			assert.Equal(t, en.New(), fallback)
			assert.ElementsMatch(t, []locales.Translator{en.New()}, supportedLocales)
			return utNewFuncMock
		})
		defer patches.Reset()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			translator, e := factory.Get("my_translator")
			require.Nil(t, translator)
			require.ErrorIs(t, e, flam.ErrLanguageNotFound)
		}))
	})

	t.Run("should correctly create a ten translator resource", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			translator, e := factory.Get("my_translator")
			require.NotNil(t, translator)
			require.NoError(t, e)
		}))
	})
}
