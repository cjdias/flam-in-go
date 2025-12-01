package tests

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_RedisBooter_Close(t *testing.T) {
	t.Run("should correctly close the app if mini redis was instantiated", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisMiniBoot, true)

		app := flam.NewApplication(config)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Close())
	})
}

func Test_RedisBooter_Boot(t *testing.T) {
	t.Run("should not boot the mini redis if not flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverMini}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.Nil(t, config.Get(
				fmt.Sprintf("%s.my_connection.host", flam.PathRedisConnections)))
		}))
	})

	t.Run("should return mini redis starting error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisMiniBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedError := errors.New("mini redis boot error")
		var m *miniredis.Miniredis
		patches := gomonkey.ApplyMethod(reflect.TypeOf(m), "Start", func(*miniredis.Miniredis) error {
			return expectedError
		})
		defer patches.Reset()

		assert.Error(t, app.Boot(), expectedError)
	})

	t.Run("should store mini redis address in all mini redis connections if booted", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisMiniBoot, true)
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverMini}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.NotNil(t, config.Get(
				fmt.Sprintf("%s.my_connection.host", flam.PathRedisConnections)))
		}))
	})
}
