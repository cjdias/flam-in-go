package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_TranslatorFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added translators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorAlphaMock := mocks.NewMockTranslator(ctrl)
		translatorZuluMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("alpha", translatorAlphaMock))
			require.NoError(t, factory.Store("zulu", translatorZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added translators and config defined translators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		translatorCharlieMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("charlie", translatorCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_TranslatorFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated translators", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.TranslatorDriverEnglish},
			"alpha": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added translators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorMock1 := mocks.NewMockTranslator(ctrl)
		translatorMock2 := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("my_translator_1", translatorMock1))
			require.NoError(t, factory.Store("my_translator_2", translatorMock2))

			assert.Equal(t, []string{"my_translator_1", "my_translator_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated translators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.TranslatorDriverEnglish},
			"alpha": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		translatorMock1 := mocks.NewMockTranslator(ctrl)
		translatorMock2 := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_translator_1", translatorMock1))
			require.NoError(t, factory.Store("my_translator_2", translatorMock2))

			assert.Equal(t, []string{"alpha", "my_translator_1", "my_translator_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_TranslatorFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathTranslators, flam.Bag{
		"ny_translator_1": flam.Bag{
			"driver": flam.TranslatorDriverEnglish}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	translatorMock := mocks.NewMockTranslator(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
		require.NoError(t, factory.Store("ny_translator_2", translatorMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_translator_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_translator_2",
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

func Test_TranslatorFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			got, e := factory.Get("my_translator")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved translator", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			got, e := factory.Get("my_translator")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_translator")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_TranslatorFactory_Store(t *testing.T) {
	t.Run("should return nil reference if translator is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.ErrorIs(t, factory.Store("my_translator", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if translator reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		translatorMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.ErrorIs(t, factory.Store("my_translator", translatorMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if translator has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.NoError(t, factory.Store("my_translator", translatorMock))
		}))
	})

	t.Run("should return duplicate resource if translator has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.NoError(t, factory.Store("my_translator", translatorMock))
			assert.ErrorIs(t, factory.Store("my_translator", translatorMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_TranslatorFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the translator is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.ErrorIs(t, factory.Remove("my_translator"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove translator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("my_translator", translatorMock))

			assert.NoError(t, factory.Remove("my_translator"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_TranslatorFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored translators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorMock1 := mocks.NewMockTranslator(ctrl)
		translatorMock2 := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("my_translator_1", translatorMock1))
			require.NoError(t, factory.Store("my_translator_2", translatorMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
