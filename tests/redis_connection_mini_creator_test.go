package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_MiniRedisConnectionCreator(t *testing.T) {
	t.Run("should ignore config without/empty host field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverMini,
				"host":   ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			got, e := factory.Get("my_connection")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})
}
