package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultValidatorCreator(t *testing.T) {
	t.Run("should ignore config without/empty parser_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":             flam.ValidatorDriverDefault,
				"error_converter_id": "my_error_converter"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.Nil(t, validator)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return parser creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":             flam.ValidatorDriverDefault,
				"parser_id":          "my_parser",
				"error_converter_id": "my_error_converter"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.Nil(t, validator)
			require.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return error converter creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":             flam.ValidatorDriverDefault,
				"parser_id":          "my_parser",
				"error_converter_id": "my_error_converter"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.Nil(t, validator)
			require.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should correctly generate the validator", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.NotNil(t, validator)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate the validator with default error converter if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorDefaultErrorConverterId, "my_converter")
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		validationErrorConverter := mocks.NewMockValidatorErrorConverter(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("my_converter", validationErrorConverter))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.NotNil(t, validator)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate the validator with default parser if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorDefaultParserId, "my_parser")
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver": flam.ValidatorDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.NotNil(t, validator)
			require.NoError(t, e)
		}))
	})
}
