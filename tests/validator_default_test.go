package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultValidator(t *testing.T) {
	t.Run("should return nil if no error was found in the data", func(t *testing.T) {
		type testStruct struct {
			Param1 string `paramId:"1"`
			Param2 string `paramId:"2"`
		}

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

			assert.Nil(t, validator.Validate(testStruct{
				Param1: "string_1",
				Param2: "string_2"}))
		}))
	})

	t.Run("should return parser errors", func(t *testing.T) {
		type testStruct struct {
			Param1 string `paramId:"1" validate:"required"`
			Param2 string `paramId:"2" validate:"max=5"`
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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

			validationErrors := validator.Validate(testStruct{
				Param2: "string_2"})

			assert.Len(t, validationErrors, 2)

			verr := validationErrors.([]flam.ValidationError)[0]
			assert.Equal(t, 1, verr.ParamId)
			assert.Equal(t, "Param1", verr.ParamName)
			assert.Equal(t, 104, verr.ErrorId)
			assert.Contains(t, verr.ErrorMessage, `Field validation for 'Param1' failed on the 'required' tag`)

			verr = validationErrors.([]flam.ValidationError)[1]
			assert.Equal(t, 2, verr.ParamId)
			assert.Equal(t, "Param2", verr.ParamName)
			assert.Equal(t, 101, verr.ErrorId)
			assert.Contains(t, verr.ErrorMessage, `Field validation for 'Param2' failed on the 'max' tag`)
		}))
	})

	t.Run("should return converted parser errors", func(t *testing.T) {
		type testStruct struct {
			Param1 string `paramId:"1" validate:"required"`
			Param2 string `paramId:"2" validate:"max=5"`
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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
				"error_converter_id": "my_converter"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		validationErrorConverter := mocks.NewMockValidatorErrorConverter(ctrl)
		validationErrorConverter.EXPECT().Convert([]flam.ValidationError{
			{
				ParamId:      1,
				ParamName:    "Param1",
				ErrorId:      104,
				ErrorMessage: `Key: 'testStruct.Param1' Error:Field validation for 'Param1' failed on the 'required' tag`},
			{
				ParamId:      2,
				ParamName:    "Param2",
				ErrorId:      101,
				ErrorMessage: `Key: 'testStruct.Param2' Error:Field validation for 'Param2' failed on the 'max' tag`}}).Return(123)
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("my_converter", validationErrorConverter))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			validator, e := factory.Get("my_validator")
			require.NotNil(t, validator)
			require.NoError(t, e)

			validationErrors := validator.Validate(testStruct{
				Param2: "string_2"})

			assert.Equal(t, 123, validationErrors.(int))
		}))
	})
}
