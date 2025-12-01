package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_DefaultRedisConnectionCreator(t *testing.T) {
	t.Run("should ignore config without/empty host field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault,
				"host":   "",
				"port":   6379}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty port field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault,
				"host":   "host",
				"port":   0}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should create with default host if none is given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault,
				"port":   6379}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			require.NotNil(t, got)
			require.NoError(t, e)
		}))
	})

	t.Run("should create with default port if none is given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault,
				"host":   "host"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			require.NotNil(t, got)
			require.NoError(t, e)
		}))
	})
}
