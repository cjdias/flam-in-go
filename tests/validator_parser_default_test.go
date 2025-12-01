package tests

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultValidationParser_Parse(t *testing.T) {
	type testStruct struct {
		Param1 string `paramId:"1"`
		Param2 string `paramId:"2"`
	}

	t.Run("should no-op if no error has been passed", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "paramId",
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			assert.Nil(t, parser.Parse(&testStruct{}, validator.ValidationErrors{}))
		}))
	})

	t.Run("should generate validation error list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "paramId",
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		fieldErrorMock := mocks.NewMockFieldError(ctrl)
		fieldErrorMock.EXPECT().StructField().Return("Param2")
		fieldErrorMock.EXPECT().Tag().Return("hostname_rfc1123")
		fieldErrorMock.EXPECT().Translate(gomock.Any()).Return("translated field error")

		validatorValidationErrors := []validator.FieldError{}
		validatorValidationErrors = append(validatorValidationErrors, fieldErrorMock)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			validationErrors := parser.Parse(testStruct{}, validatorValidationErrors)

			assert.Len(t, validationErrors, 1)
			assert.Equal(t, validationErrors[0].ParamId, 2)
			assert.Equal(t, validationErrors[0].ParamName, "Param2")
			assert.Equal(t, validationErrors[0].ErrorId, 22)
			assert.Equal(t, validationErrors[0].ErrorMessage, "translated field error")
		}))
	})
}

func Test_DefaultValidationParser_AddTagCode(t *testing.T) {
	type testStruct struct {
		Param1 string `paramId:"1"`
		Param2 string `paramId:"2"`
	}

	t.Run("should generate validation error list with added tag code", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"annotation":    "paramId",
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		fieldErrorMock := mocks.NewMockFieldError(ctrl)
		fieldErrorMock.EXPECT().StructField().Return("Param2")
		fieldErrorMock.EXPECT().Tag().Return("my_tag")
		fieldErrorMock.EXPECT().Translate(gomock.Any()).Return("translated field error")

		validatorValidationErrors := []validator.FieldError{}
		validatorValidationErrors = append(validatorValidationErrors, fieldErrorMock)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			parser.AddTagCode("my_tag", 12345)

			validationErrors := parser.Parse(testStruct{}, validatorValidationErrors)

			assert.Len(t, validationErrors, 1)
			assert.Equal(t, validationErrors[0].ParamId, 2)
			assert.Equal(t, validationErrors[0].ParamName, "Param2")
			assert.Equal(t, validationErrors[0].ErrorId, 12345)
			assert.Equal(t, validationErrors[0].ErrorMessage, "translated field error")
		}))
	})
}
