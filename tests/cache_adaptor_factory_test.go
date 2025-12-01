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

func Test_CacheAdaptorFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added adaptors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorAlphaMock := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorZuluMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			require.NoError(t, factory.Store("alpha", cacheAdaptorAlphaMock))
			require.NoError(t, factory.Store("zulu", cacheAdaptorZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added adaptors and config defined adaptors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheAdaptorCharlieMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			require.NoError(t, factory.Store("charlie", cacheAdaptorCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_CacheAdaptorFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated adaptors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheAdaptorZuluMock := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorMockAlpha := mocks.NewMockCacheAdaptor(ctrl)

		cacheAdaptorCreatorMock := mocks.NewMockCacheAdaptorCreator(ctrl)
		cacheAdaptorCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		cacheAdaptorCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		cacheAdaptorCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(cacheAdaptorZuluMock, nil)
		cacheAdaptorCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(cacheAdaptorMockAlpha, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheAdaptorCreator {
			return cacheAdaptorCreatorMock
		}, dig.Group(flam.CacheAdaptorCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added adaptors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorMock1 := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorMock2 := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			require.NoError(t, factory.Store("my_adaptor_1", cacheAdaptorMock1))
			require.NoError(t, factory.Store("my_adaptor_2", cacheAdaptorMock2))

			assert.Equal(t, []string{"my_adaptor_1", "my_adaptor_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated adaptors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheAdaptorZuluMock := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorMockAlpha := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorMock1 := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorMock2 := mocks.NewMockCacheAdaptor(ctrl)

		cacheAdaptorCreatorMock := mocks.NewMockCacheAdaptorCreator(ctrl)
		cacheAdaptorCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		cacheAdaptorCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		cacheAdaptorCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(cacheAdaptorZuluMock, nil)
		cacheAdaptorCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(cacheAdaptorMockAlpha, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheAdaptorCreator {
			return cacheAdaptorCreatorMock
		}, dig.Group(flam.CacheAdaptorCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_adaptor_1", cacheAdaptorMock1))
			require.NoError(t, factory.Store("my_adaptor_2", cacheAdaptorMock2))

			assert.Equal(t, []string{"alpha", "my_adaptor_1", "my_adaptor_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_CacheAdaptorFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
		"ny_adaptor_1": flam.Bag{}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
		require.NoError(t, factory.Store("ny_adaptor_2", cacheAdaptorMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_adaptor_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_adaptor_2",
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

func Test_CacheAdaptorFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved adaptor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

		cacheAdaptorCreatorMock := mocks.NewMockCacheAdaptorCreator(ctrl)
		cacheAdaptorCreatorMock.EXPECT().Accept(flam.Bag{"id": "my_adaptor", "driver": "mock"}).Return(true)
		cacheAdaptorCreatorMock.EXPECT().Create(flam.Bag{"id": "my_adaptor", "driver": "mock"}).Return(cacheAdaptorMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheAdaptorCreator {
			return cacheAdaptorCreatorMock
		}, dig.Group(flam.CacheAdaptorCreatorGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_adaptor")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_CacheAdaptorFactory_Store(t *testing.T) {
	t.Run("should return nil reference if adaptor is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.ErrorIs(t, factory.Store("my_adaptor", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if adaptor reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.ErrorIs(t, factory.Store("my_adaptor", cacheAdaptorMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if adaptor has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.NoError(t, factory.Store("my_adaptor", cacheAdaptorMock))
		}))
	})

	t.Run("should return duplicate resource if adaptor has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.NoError(t, factory.Store("my_adaptor", cacheAdaptorMock))
			assert.ErrorIs(t, factory.Store("my_adaptor", cacheAdaptorMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_CacheAdaptorFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the adaptor is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.ErrorIs(t, factory.Remove("my_adaptor"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove adaptor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			require.NoError(t, factory.Store("my_adaptor", cacheAdaptorMock))

			assert.NoError(t, factory.Remove("my_adaptor"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_CacheAdaptorFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored adaptors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorMock1 := mocks.NewMockCacheAdaptor(ctrl)
		cacheAdaptorMock2 := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			require.NoError(t, factory.Store("my_adaptor_1", cacheAdaptorMock1))
			require.NoError(t, factory.Store("my_adaptor_2", cacheAdaptorMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
