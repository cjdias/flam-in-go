package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ConfigParserFactory_Close(t *testing.T) {
	t.Run("should correctly close stored parsers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		configParserMock := mocks.NewMockConfigParser(ctrl)
		configParserMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.NoError(t, factory.Store("my_resource", configParserMock))
		}))

		assert.NoError(t, app.Close())
	})
}

func Test_ConfigParserFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added parsers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserAlphaMock := mocks.NewMockConfigParser(ctrl)
		configParserAlphaMock.EXPECT().Close().Return(nil)

		configParserZuluMock := mocks.NewMockConfigParser(ctrl)
		configParserZuluMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			require.NoError(t, factory.Store("alpha", configParserAlphaMock))
			require.NoError(t, factory.Store("zulu", configParserZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added parsers and config defined parsers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configParserCharlieMock := mocks.NewMockConfigParser(ctrl)
		configParserCharlieMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			require.NoError(t, factory.Store("charlie", configParserCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_ConfigParserFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated parsers", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.ConfigParserDriverJson},
			"alpha": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added parsers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserMock1 := mocks.NewMockConfigParser(ctrl)
		configParserMock1.EXPECT().Close().Return(nil)

		configParserMock2 := mocks.NewMockConfigParser(ctrl)
		configParserMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			require.NoError(t, factory.Store("my_parser_1", configParserMock1))
			require.NoError(t, factory.Store("my_parser_2", configParserMock2))

			assert.Equal(t, []string{"my_parser_1", "my_parser_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated parsers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.ConfigParserDriverJson},
			"alpha": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configParserMock1 := mocks.NewMockConfigParser(ctrl)
		configParserMock1.EXPECT().Close().Return(nil)

		configParserMock2 := mocks.NewMockConfigParser(ctrl)
		configParserMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_parser_1", configParserMock1))
			require.NoError(t, factory.Store("my_parser_2", configParserMock2))

			assert.Equal(t, []string{"alpha", "my_parser_1", "my_parser_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_ConfigParserFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathConfigParsers, flam.Bag{
		"ny_parser_1": flam.Bag{
			"driver": flam.ConfigParserDriverJson}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	configParserMock := mocks.NewMockConfigParser(ctrl)
	configParserMock.EXPECT().Close().Return(nil)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
		require.NoError(t, factory.Store("ny_parser_2", configParserMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_parser_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_parser_2",
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

func Test_ConfigParserFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			got, e := factory.Get("my_parser")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved parser", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			got, e := factory.Get("my_parser")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_parser")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_ConfigParserFactory_Store(t *testing.T) {
	t.Run("should return nil reference if parser is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.ErrorIs(t, factory.Store("my_parser", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if parser reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configParserMock := mocks.NewMockConfigParser(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.ErrorIs(t, factory.Store("my_parser", configParserMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if parser has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserMock := mocks.NewMockConfigParser(ctrl)
		configParserMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.NoError(t, factory.Store("my_parser", configParserMock))
		}))
	})

	t.Run("should return duplicate resource if parser has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserMock := mocks.NewMockConfigParser(ctrl)
		configParserMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.NoError(t, factory.Store("my_parser", configParserMock))
			assert.ErrorIs(t, factory.Store("my_parser", configParserMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_ConfigParserFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the parser is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.ErrorIs(t, factory.Remove("my_parser"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove parser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserMock := mocks.NewMockConfigParser(ctrl)
		configParserMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			require.NoError(t, factory.Store("my_parser", configParserMock))

			assert.NoError(t, factory.Remove("my_parser"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_ConfigParserFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored parsers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserMock1 := mocks.NewMockConfigParser(ctrl)
		configParserMock1.EXPECT().Close().Return(nil)

		configParserMock2 := mocks.NewMockConfigParser(ctrl)
		configParserMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			require.NoError(t, factory.Store("my_parser_1", configParserMock1))
			require.NoError(t, factory.Store("my_parser_2", configParserMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
