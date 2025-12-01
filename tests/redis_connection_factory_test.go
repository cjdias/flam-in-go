package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_RedisConnectionFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added connections", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionAlphaMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionAlphaMock.EXPECT().Close().Return(nil)

		redisConnectionZuluMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionZuluMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("alpha", redisConnectionAlphaMock))
			require.NoError(t, factory.Store("zulu", redisConnectionZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added connections and config defined connections", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		redisConnectionCharlieMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionCharlieMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("charlie", redisConnectionCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_RedisConnectionFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated connections", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault},
			"alpha": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added connections", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionMock1 := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock1.EXPECT().Close().Return(nil)

		redisConnectionMock2 := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection_1", redisConnectionMock1))
			require.NoError(t, factory.Store("my_connection_2", redisConnectionMock2))

			assert.Equal(t, []string{"my_connection_1", "my_connection_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated connections", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault},
			"alpha": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		redisConnectionMock1 := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock1.EXPECT().Close().Return(nil)

		redisConnectionMock2 := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_connection_1", redisConnectionMock1))
			require.NoError(t, factory.Store("my_connection_2", redisConnectionMock2))

			assert.Equal(t, []string{"alpha", "my_connection_1", "my_connection_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_RedisConnectionFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathRedisConnections, flam.Bag{
		"ny_connection_1": flam.Bag{
			"driver": flam.RedisConnectionDriverDefault}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
	redisConnectionMock.EXPECT().Close().Return(nil)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
		require.NoError(t, factory.Store("ny_connection_2", redisConnectionMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_connection_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_connection_2",
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

func Test_RedisConnectionFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved connection", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_connection")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_RedisConnectionFactory_Store(t *testing.T) {
	t.Run("should return nil reference if connection is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.ErrorIs(t, factory.Store("my_connection", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if connection reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.ErrorIs(t, factory.Store("my_connection", redisConnectionMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if connection has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))
	})

	t.Run("should return duplicate resource if connection has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.NoError(t, factory.Store("my_connection", redisConnectionMock))
			assert.ErrorIs(t, factory.Store("my_connection", redisConnectionMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_RedisConnectionFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the connection is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.ErrorIs(t, factory.Remove("my_connection"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))

			assert.NoError(t, factory.Remove("my_connection"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_RedisConnectionFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored connections", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionMock1 := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock1.EXPECT().Close().Return(nil)

		redisConnectionMock2 := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection_1", redisConnectionMock1))
			require.NoError(t, factory.Store("my_connection_2", redisConnectionMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
