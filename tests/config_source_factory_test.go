package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ConfigSourceFactory_Close(t *testing.T) {
	t.Run("should correctly close stored sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.NoError(t, factory.Store("my_source", configSourceMock))
		}))

		assert.NoError(t, app.Close())
	})
}

func Test_ConfigSourceFactory_Available(t *testing.T) {
	t.Run("should return a list with ony the app loading source when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.Equal(t, []string{"__app"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.Equal(t, []string{"__app", "alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceAlphaMock := mocks.NewMockConfigSource(ctrl)
		configSourceAlphaMock.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceAlphaMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceAlphaMock.EXPECT().Close().Return(nil)

		configSourceZuluMock := mocks.NewMockConfigSource(ctrl)
		configSourceZuluMock.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceZuluMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceZuluMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("alpha", configSourceAlphaMock))
			require.NoError(t, factory.Store("zulu", configSourceZuluMock))

			assert.Equal(t, []string{"__app", "alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added sources and config defined sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configSourceCharlieMock := mocks.NewMockConfigSource(ctrl)
		configSourceCharlieMock.EXPECT().GetPriority().Return(0)
		configSourceCharlieMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceCharlieMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("charlie", configSourceCharlieMock))

			assert.Equal(t, []string{"__app", "alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_ConfigSourceFactory_Stored(t *testing.T) {
	t.Run("should return a list with only app source of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.Equal(t, []string{"__app"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated sources", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv},
			"alpha": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"__app", "alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock1 := mocks.NewMockConfigSource(ctrl)
		configSourceMock1.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceMock1.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceMock1.EXPECT().Close().Return(nil)

		configSourceMock2 := mocks.NewMockConfigSource(ctrl)
		configSourceMock2.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceMock2.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_source_1", configSourceMock1))
			require.NoError(t, factory.Store("my_source_2", configSourceMock2))

			assert.Equal(t, []string{"__app", "my_source_1", "my_source_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv},
			"alpha": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configSourceMock1 := mocks.NewMockConfigSource(ctrl)
		configSourceMock1.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceMock1.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceMock1.EXPECT().Close().Return(nil)

		configSourceMock2 := mocks.NewMockConfigSource(ctrl)
		configSourceMock2.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceMock2.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_source_1", configSourceMock1))
			require.NoError(t, factory.Store("my_source_2", configSourceMock2))

			assert.Equal(t, []string{"__app", "alpha", "my_source_1", "my_source_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_ConfigSourceFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathConfigSources, flam.Bag{
		"ny_source_1": flam.Bag{
			"driver": flam.ConfigSourceDriverEnv}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	configSourceMock := mocks.NewMockConfigSource(ctrl)
	configSourceMock.EXPECT().GetPriority().Return(0)
	configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
	configSourceMock.EXPECT().Close().Return(nil)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
		require.NoError(t, factory.Store("ny_source_2", configSourceMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_source_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_source_2",
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

func Test_ConfigSourceFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved source", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_source")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})

	t.Run("should add the loaded source config data to the manager", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field": "value"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().GetPriority().Return(0)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		configSourceCreatorMock := mocks.NewMockConfigSourceCreator(ctrl)
		configSourceCreatorMock.EXPECT().Accept(flam.Bag{"id": "my_source", "driver": "mock"}).Return(true)
		configSourceCreatorMock.EXPECT().Create(flam.Bag{"id": "my_source", "driver": "mock"}).Return(configSourceMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.ConfigSourceCreator {
			return configSourceCreatorMock
		}, dig.Group(flam.ConfigSourceCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			got, e := factory.Get("my_source")
			require.Same(t, configSourceMock, got)
			require.NoError(t, e)

			assert.Equal(t, "value", config.Get("field"))
		}))
	})
}

func Test_ConfigSourceFactory_Store(t *testing.T) {
	t.Run("should return nil reference if source is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.ErrorIs(t, factory.Store("my_source", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if source reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.ErrorIs(t, factory.Store("my_source", configSourceMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if source has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().GetPriority().Return(0)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.NoError(t, factory.Store("my_source", configSourceMock))
		}))
	})

	t.Run("should return duplicate resource if source has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().GetPriority().Return(0)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.NoError(t, factory.Store("my_source", configSourceMock))
			assert.ErrorIs(t, factory.Store("my_source", configSourceMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should add the stored source config data to the manager", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field": "value"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().GetPriority().Return(0)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			e := factory.Store("my_source", configSourceMock)
			require.NoError(t, e)

			assert.Equal(t, "value", config.Get("field"))
		}))
	})
}

func Test_ConfigSourceFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the source is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.ErrorIs(t, factory.Remove("my_source"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().GetPriority().Return(0)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.NoError(t, factory.Remove("my_source"))

			assert.Equal(t, []string{"__app"}, factory.Stored())
		}))
	})

	t.Run("should remove source data from the config manager", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field": "value"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.Equal(t, "value", config.Get("field"))

			assert.NoError(t, factory.Remove("my_source"))
			assert.False(t, config.Has("field"))
		}))
	})
}

func Test_ConfigSourceFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock1 := mocks.NewMockConfigSource(ctrl)
		configSourceMock1.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceMock1.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceMock1.EXPECT().Close().Return(nil)

		configSourceMock2 := mocks.NewMockConfigSource(ctrl)
		configSourceMock2.EXPECT().GetPriority().Return(0).AnyTimes()
		configSourceMock2.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).AnyTimes()
		configSourceMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_source_1", configSourceMock1))
			require.NoError(t, factory.Store("my_source_2", configSourceMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should remove source data from the config manager", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field": "value"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.Equal(t, "value", config.Get("field"))

			assert.NoError(t, factory.RemoveAll())
			assert.False(t, config.Has("field"))
		}))
	})
}

func Test_ConfigSourceFactory_Reload(t *testing.T) {
	t.Run("should no-op if none source is an observable source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.NoError(t, factory.Reload())
		}))
	})

	t.Run("should return any observable source reload error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("close failed")
		configSourceMock := mocks.NewMockObservableConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Reload().Return(false, expectedErr)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.ErrorIs(t, factory.Reload(), expectedErr)
		}))
	})

	t.Run("should no-op if no observable source notify change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockObservableConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Reload().Return(false, nil)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.NoError(t, factory.Reload())
		}))
	})

	t.Run("should update config if any source was successfully reloaded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockObservableConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{"field": "value1"})
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{"field": "value2"})
		configSourceMock.EXPECT().Reload().Return(true, nil)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.Equal(t, "value1", config.Get("field"))

			assert.NoError(t, factory.Reload())
			require.Equal(t, "value2", config.Get("field"))
		}))
	})
}

func Test_ConfigSourceFactory_SetPriority(t *testing.T) {
	t.Run("should return source not found for an invalid id", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.ErrorIs(t, factory.SetPriority("invalid", 1), flam.ErrUnknownResource)
		}))
	})

	t.Run("should update the source priority and update the config manager", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		firstConfigSourceMock := mocks.NewMockConfigSource(ctrl)
		firstConfigSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{"field": "value1"}).AnyTimes()
		firstConfigSourceMock.EXPECT().GetPriority().Return(0)
		firstConfigSourceMock.EXPECT().SetPriority(2)
		firstConfigSourceMock.EXPECT().GetPriority().Return(2)
		firstConfigSourceMock.EXPECT().Close().Return(nil)

		secondConfigSourceMock := mocks.NewMockConfigSource(ctrl)
		secondConfigSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{"field": "value2"}).AnyTimes()
		secondConfigSourceMock.EXPECT().GetPriority().Return(1).Times(2)
		secondConfigSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source_1", firstConfigSourceMock))
			require.NoError(t, factory.Store("my_source_2", secondConfigSourceMock))

			require.Equal(t, "value2", config.Get("field"))

			assert.NoError(t, factory.SetPriority("my_source_1", 2))
			assert.Equal(t, "value1", config.Get("field"))
		}))
	})
}
