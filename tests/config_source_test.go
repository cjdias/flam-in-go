package tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_ConfigSource_GetPriority(t *testing.T) {
	config := flam.Bag{}
	_ = config.Set(flam.PathConfigBoot, true)
	_ = config.Set(flam.PathConfigSources, flam.Bag{
		"my_source": flam.Bag{
			"driver":   flam.ConfigSourceDriverEnv,
			"priority": 123,
			"files":    []string{"./testdata/env"}}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	require.NoError(t, app.Boot())

	assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
		source, e := factory.Get("my_source")
		require.NotNil(t, source)
		require.NoError(t, e)

		assert.Equal(t, 123, source.GetPriority())
	}))
}

func Test_ConfigSource_SetPriority(t *testing.T) {
	config := flam.Bag{}
	_ = config.Set(flam.PathConfigBoot, true)
	_ = config.Set(flam.PathConfigSources, flam.Bag{
		"my_source": flam.Bag{
			"driver":   flam.ConfigSourceDriverEnv,
			"priority": 123,
			"files":    []string{"./testdata/env"}}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	require.NoError(t, app.Boot())

	assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
		source, e := factory.Get("my_source")
		require.NotNil(t, source)
		require.NoError(t, e)

		assert.Equal(t, 123, source.GetPriority())

		source.SetPriority(234)
		assert.Equal(t, 234, source.GetPriority())
	}))
}

func Test_ConfigSource_Get(t *testing.T) {
	_ = os.Setenv("APP_DATA", "value")
	defer func() { _ = os.Setenv("APP_DATA", "") }()

	config := flam.Bag{}
	_ = config.Set(flam.PathConfigBoot, true)
	_ = config.Set(flam.PathConfigSources, flam.Bag{
		"my_source": flam.Bag{
			"driver":   flam.ConfigSourceDriverEnv,
			"priority": 123,
			"files":    []string{"./testdata/env"},
			"mappings": flam.Bag{
				"APP_DATA": "data"}}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	require.NoError(t, app.Boot())

	assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
		source, e := factory.Get("my_source")
		require.NotNil(t, source)
		require.NoError(t, e)

		assert.Equal(t, "value", source.Get("data"))
	}))
}
