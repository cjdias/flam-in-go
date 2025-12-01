package tests

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_FileConfigSource(t *testing.T) {
	t.Run("should return file opening error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverFile,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "/testdata/config",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", afero.NewMemMapFs()))
		}))

		assert.ErrorContains(t, app.Boot(), "file does not exist")
	})

	t.Run("should return file parsing error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverFile,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "/testdata/config",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/config")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.WriteString("{")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.ErrorContains(t, app.Boot(), "yaml: line 1: did not find expected node content")
	})

	t.Run("should correctly load file source", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverFile,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "/testdata/config",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/config")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.WriteString("field: value")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			got, e := factory.Get("my_source")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			assert.Equal(t, "value", got.Get("field"))
			assert.Equal(t, "value", config.Get("field"))
		}))
	})
}
