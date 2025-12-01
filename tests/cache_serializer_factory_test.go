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

func Test_CacheSerializerFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerAlphaMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerZuluMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("alpha", cacheSerializerAlphaMock))
			require.NoError(t, factory.Store("zulu", cacheSerializerZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added serializers and config defined serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheSerializerCharlieMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("charlie", cacheSerializerCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_CacheSerializerFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheSerializerZuluMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMockAlpha := mocks.NewMockCacheSerializer(ctrl)

		cacheSerializerCreatorMock := mocks.NewMockCacheSerializerCreator(ctrl)
		cacheSerializerCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		cacheSerializerCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		cacheSerializerCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(cacheSerializerZuluMock, nil)
		cacheSerializerCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(cacheSerializerMockAlpha, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheSerializerCreator {
			return cacheSerializerCreatorMock
		}, dig.Group(flam.CacheSerializerCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerMock1 := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock2 := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer_1", cacheSerializerMock1))
			require.NoError(t, factory.Store("my_serializer_2", cacheSerializerMock2))

			assert.Equal(t, []string{"my_serializer_1", "my_serializer_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"zulu": flam.Bag{
				"driver": "mock"},
			"alpha": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheSerializerZuluMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMockAlpha := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock1 := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock2 := mocks.NewMockCacheSerializer(ctrl)

		cacheSerializerCreatorMock := mocks.NewMockCacheSerializerCreator(ctrl)
		cacheSerializerCreatorMock.EXPECT().Accept(flam.Bag{"id": "zulu", "driver": "mock"}).Return(true)
		cacheSerializerCreatorMock.EXPECT().Accept(flam.Bag{"id": "alpha", "driver": "mock"}).Return(true)
		cacheSerializerCreatorMock.EXPECT().Create(flam.Bag{"id": "zulu", "driver": "mock"}).Return(cacheSerializerZuluMock, nil)
		cacheSerializerCreatorMock.EXPECT().Create(flam.Bag{"id": "alpha", "driver": "mock"}).Return(cacheSerializerMockAlpha, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheSerializerCreator {
			return cacheSerializerCreatorMock
		}, dig.Group(flam.CacheSerializerCreatorGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_serializer_1", cacheSerializerMock1))
			require.NoError(t, factory.Store("my_serializer_2", cacheSerializerMock2))

			assert.Equal(t, []string{"alpha", "my_serializer_1", "my_serializer_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_CacheSerializerFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathCacheSerializers, flam.Bag{
		"ny_serializer_1": flam.Bag{}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
		require.NoError(t, factory.Store("ny_serializer_2", cacheSerializerMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_serializer_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_serializer_2",
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

func Test_CacheSerializerFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			got, e := factory.Get("my_serializer")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved serializer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": "mock"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

		cacheSerializerCreatorMock := mocks.NewMockCacheSerializerCreator(ctrl)
		cacheSerializerCreatorMock.EXPECT().Accept(flam.Bag{"id": "my_serializer", "driver": "mock"}).Return(true)
		cacheSerializerCreatorMock.EXPECT().Create(flam.Bag{"id": "my_serializer", "driver": "mock"}).Return(cacheSerializerMock, nil)
		require.NoError(t, app.Container().Provide(func() flam.CacheSerializerCreator {
			return cacheSerializerCreatorMock
		}, dig.Group(flam.CacheSerializerCreatorGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			got, e := factory.Get("my_serializer")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_serializer")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_CacheSerializerFactory_Store(t *testing.T) {
	t.Run("should return nil reference if serializer is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.ErrorIs(t, factory.Store("my_serializer", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if serializer reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.ErrorIs(t, factory.Store("my_serializer", cacheSerializerMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if serializer has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))
	})

	t.Run("should return duplicate resource if serializer has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
			assert.ErrorIs(t, factory.Store("my_serializer", cacheSerializerMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_CacheSerializerFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the serializer is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.ErrorIs(t, factory.Remove("my_serializer"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove serializer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))

			assert.NoError(t, factory.Remove("my_serializer"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_CacheSerializerFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerMock1 := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock2 := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer_1", cacheSerializerMock1))
			require.NoError(t, factory.Store("my_serializer_2", cacheSerializerMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
