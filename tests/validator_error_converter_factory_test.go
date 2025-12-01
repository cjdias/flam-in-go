package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ValidatorErrorConverterFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added converters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		converterAlphaMock := mocks.NewMockValidatorErrorConverter(ctrl)
		converterZuluMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("alpha", converterAlphaMock))
			require.NoError(t, factory.Store("zulu", converterZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added converters and config defined converters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		converterCharlieMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("charlie", converterCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_ValidatorErrorConverterFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated converters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		errorConverterZuluMock := mocks.NewMockValidatorErrorConverter(ctrl)
		errorConverterAlphaMock := mocks.NewMockValidatorErrorConverter(ctrl)

		errorConverterCreatorMock := mocks.NewMockValidatorErrorConverterCreator(ctrl)
		errorConverterCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		errorConverterCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(errorConverterZuluMock, nil)
		errorConverterCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		errorConverterCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(errorConverterAlphaMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.ValidatorErrorConverterCreator {
			return errorConverterCreatorMock
		}, dig.Group(flam.ValidatorErrorConverterCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added converters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		converterMock1 := mocks.NewMockValidatorErrorConverter(ctrl)
		converterMock2 := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("my_converter_1", converterMock1))
			require.NoError(t, factory.Store("my_converter_2", converterMock2))

			assert.Equal(t, []string{"my_converter_1", "my_converter_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated converters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		converterMock1 := mocks.NewMockValidatorErrorConverter(ctrl)
		converterMock2 := mocks.NewMockValidatorErrorConverter(ctrl)

		errorConverterZuluMock := mocks.NewMockValidatorErrorConverter(ctrl)
		errorConverterAlphaMock := mocks.NewMockValidatorErrorConverter(ctrl)

		errorConverterCreatorMock := mocks.NewMockValidatorErrorConverterCreator(ctrl)
		errorConverterCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		errorConverterCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(errorConverterZuluMock, nil)
		errorConverterCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		errorConverterCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(errorConverterAlphaMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.ValidatorErrorConverterCreator {
			return errorConverterCreatorMock
		}, dig.Group(flam.ValidatorErrorConverterCreatorGroup)))

		translatorMock := mocks.NewMockTranslator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("my_translator", translatorMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_converter_1", converterMock1))
			require.NoError(t, factory.Store("my_converter_2", converterMock2))

			assert.Equal(t, []string{"alpha", "my_converter_1", "my_converter_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_ValidatorErrorConverterFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
		"ny_converter_1": flam.Bag{
			"driver": "mock"}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	converterMock := mocks.NewMockValidatorErrorConverter(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
		require.NoError(t, factory.Store("ny_converter_2", converterMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_converter_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_converter_2",
				expected: true},
			{
				name:     "non-existent entry",
				id:       "nonexistent",
				expected: false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, factory.Has(tc.id))
			})
		}
	}))
}

func Test_ValidatorErrorConverterFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"my_converter": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			got, e := factory.Get("my_converter")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved converter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"my_converter": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		errorConverterMock := mocks.NewMockValidatorErrorConverter(ctrl)

		errorConverterCreatorMock := mocks.NewMockValidatorErrorConverterCreator(ctrl)
		errorConverterCreatorMock.EXPECT().Accept(flam.Bag{"id": "my_converter", "driver": "mock"}).Return(true)
		errorConverterCreatorMock.EXPECT().Create(flam.Bag{"id": "my_converter", "driver": "mock"}).Return(errorConverterMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.ValidatorErrorConverterCreator {
			return errorConverterCreatorMock
		}, dig.Group(flam.ValidatorErrorConverterCreatorGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			got, e := factory.Get("my_converter")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_converter")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_ValidatorErrorConverterFactory_Store(t *testing.T) {
	t.Run("should return nil reference if converter is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.ErrorIs(t, factory.Store("my_converter", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if converter reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidatorErrorConverters, flam.Bag{
			"my_converter": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		converterMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.ErrorIs(t, factory.Store("my_converter", converterMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if converter has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		converterMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.NoError(t, factory.Store("my_converter", converterMock))
		}))
	})

	t.Run("should return duplicate resource if converter has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		converterMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.NoError(t, factory.Store("my_converter", converterMock))
			assert.ErrorIs(t, factory.Store("my_converter", converterMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_ValidatorErrorConverterFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the converter is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.ErrorIs(t, factory.Remove("my_converter"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove converter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		converterMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("my_converter", converterMock))

			assert.NoError(t, factory.Remove("my_converter"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_ValidatorErrorConverterFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored converters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		converterMock1 := mocks.NewMockValidatorErrorConverter(ctrl)
		converterMock2 := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("my_converter_1", converterMock1))
			require.NoError(t, factory.Store("my_converter_2", converterMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
