package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_envConfigSourceCreator(t *testing.T) {
	t.Run("should correctly instantiate the source with default values", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			source, e := factory.Get("my_source")
			assert.NotNil(t, source)
			assert.NoError(t, e)

			assert.Equal(t, 0, source.GetPriority())
			assert.Nil(t, source.Get("data"))
		}))
	})

	t.Run("should correctly instantiate the source with selected priority", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":   flam.ConfigSourceDriverEnv,
				"priority": 100}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			source, e := factory.Get("my_source")
			assert.NotNil(t, source)
			assert.NoError(t, e)

			assert.Equal(t, 100, source.GetPriority())
		}))
	})
}
