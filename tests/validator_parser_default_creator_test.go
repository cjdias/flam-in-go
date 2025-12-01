package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_DefaultValidationParserCreator(t *testing.T) {
	t.Run("should ignore config without/empty annotation field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "",
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.Nil(t, parser)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty translator_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "param",
				"translator_id": ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.Nil(t, parser)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return translator creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "param",
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.Nil(t, parser)
			require.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should correctly generate the parser", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "param",
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate the parser with default annotation if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorDefaultAnnotation, "param")
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate the parser with default translator_id if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorDefaultTranslatorId, "my_translator")
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)
		}))
	})
}
