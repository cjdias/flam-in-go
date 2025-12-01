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

func Test_CacheKeyGeneratorFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added generators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorAlphaMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorZuluMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("alpha", cacheKeyGeneratorAlphaMock))
			require.NoError(t, factory.Store("zulu", cacheKeyGeneratorZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added generators and config defined generators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorCharlieMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("charlie", cacheKeyGeneratorCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_CacheKeyGeneratorFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated generators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorZuluMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMockAlpha := mocks.NewMockCacheKeyGenerator(ctrl)

		cacheKeyGeneratorCreatorMock := mocks.NewMockCacheKeyGeneratorCreator(ctrl)
		cacheKeyGeneratorCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		cacheKeyGeneratorCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		cacheKeyGeneratorCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(cacheKeyGeneratorZuluMock, nil)
		cacheKeyGeneratorCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(cacheKeyGeneratorMockAlpha, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheKeyGeneratorCreator {
			return cacheKeyGeneratorCreatorMock
		}, dig.Group(flam.CacheKeyGeneratorCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added generators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock1 := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock2 := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_generator_1", cacheKeyGeneratorMock1))
			require.NoError(t, factory.Store("my_generator_2", cacheKeyGeneratorMock2))

			assert.Equal(t, []string{"my_generator_1", "my_generator_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated generators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorZuluMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMockAlpha := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock1 := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock2 := mocks.NewMockCacheKeyGenerator(ctrl)

		cacheKeyGeneratorCreatorMock := mocks.NewMockCacheKeyGeneratorCreator(ctrl)
		cacheKeyGeneratorCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		cacheKeyGeneratorCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		cacheKeyGeneratorCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(cacheKeyGeneratorZuluMock, nil)
		cacheKeyGeneratorCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(cacheKeyGeneratorMockAlpha, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheKeyGeneratorCreator {
			return cacheKeyGeneratorCreatorMock
		}, dig.Group(flam.CacheKeyGeneratorCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_generator_1", cacheKeyGeneratorMock1))
			require.NoError(t, factory.Store("my_generator_2", cacheKeyGeneratorMock2))

			assert.Equal(t, []string{"alpha", "my_generator_1", "my_generator_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_CacheKeyGeneratorFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
		"ny_generator_1": flam.Bag{}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
		require.NoError(t, factory.Store("ny_generator_2", cacheKeyGeneratorMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_generator_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_generator_2",
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

func Test_CacheKeyGeneratorFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_generator": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			got, e := factory.Get("my_generator")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved generator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_generator": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

		cacheKeyGeneratorCreatorMock := mocks.NewMockCacheKeyGeneratorCreator(ctrl)
		cacheKeyGeneratorCreatorMock.EXPECT().Accept(flam.Bag{"id": "my_generator", "driver": "mock"}).Return(true)
		cacheKeyGeneratorCreatorMock.EXPECT().Create(flam.Bag{"id": "my_generator", "driver": "mock"}).Return(cacheKeyGeneratorMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheKeyGeneratorCreator {
			return cacheKeyGeneratorCreatorMock
		}, dig.Group(flam.CacheKeyGeneratorCreatorGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			got, e := factory.Get("my_generator")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_generator")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_CacheKeyGeneratorFactory_Store(t *testing.T) {
	t.Run("should return nil reference if generator is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.ErrorIs(t, factory.Store("my_generator", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if generator reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_generator": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.ErrorIs(t, factory.Store("my_generator", cacheKeyGeneratorMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if generator has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.NoError(t, factory.Store("my_generator", cacheKeyGeneratorMock))
		}))
	})

	t.Run("should return duplicate resource if generator has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.NoError(t, factory.Store("my_generator", cacheKeyGeneratorMock))
			assert.ErrorIs(t, factory.Store("my_generator", cacheKeyGeneratorMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_CacheKeyGeneratorFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the generator is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.ErrorIs(t, factory.Remove("my_generator"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove generator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_generator", cacheKeyGeneratorMock))

			assert.NoError(t, factory.Remove("my_generator"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_CacheKeyGeneratorFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored generators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock1 := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock2 := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_generator_1", cacheKeyGeneratorMock1))
			require.NoError(t, factory.Store("my_generator_2", cacheKeyGeneratorMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
