package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_envConfigSource(t *testing.T) {
	t.Run("should return file open error if the selected source file does not exists", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv,
				"files":  []string{"./testdata/inexistent"},
				"mappings": flam.Bag{
					"ENV_FILE_FIELD": "data"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			source, e := factory.Get("my_source")
			assert.Nil(t, source)
			assert.ErrorContains(t, e, "no such file or directory")
		}))
	})

	t.Run("should load the source with selected env files", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv,
				"files":  []string{"./testdata/env"},
				"mappings": flam.Bag{
					"ENV_FILE_FIELD": "data"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			source, e := factory.Get("my_source")
			assert.NotNil(t, source)
			assert.NoError(t, e)

			assert.Equal(t, "file_value", source.Get("data"))
		}))
	})

	t.Run("should load the source with passed mappings", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv,
				"files":  []string{"./testdata/env"},
				"mappings": flam.Bag{
					"ENV_FILE_FIELD":            "env.data",
					"ENV_FILE_FIELD_INEXISTENT": "env.inexistent"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			source, e := factory.Get("my_source")
			assert.NotNil(t, source)
			assert.NoError(t, e)

			assert.Equal(t, "file_value", source.Get("env.data"))
			assert.Nil(t, source.Get("env.inexistent"))
		}))
	})

	t.Run("should load the source but ignore not mapped values", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv,
				"files":  []string{"./testdata/env"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			source, e := factory.Get("my_source")
			assert.NotNil(t, source)
			assert.NoError(t, e)

			assert.Nil(t, source.Get("data"))
		}))
	})
}
