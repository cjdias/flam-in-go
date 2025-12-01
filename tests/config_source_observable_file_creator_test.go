package tests

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_ObservableFileConfigSourceCreator(t *testing.T) {
	t.Run("should ignore config without/empty disk_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty path field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty parser_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should return disk retrieval error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should return parser retrieval error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", afero.NewMemMapFs()))
		}))

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should generate with default priority if not defined", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultPriority, 123)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "/testdata/config"}})

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

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Equal(t, 123, got.GetPriority())
		}))
	})

	t.Run("should generate with default disk if not defined", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultFileDiskId, "my_disk")
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"parser_id": "my_parser",
				"path":      "/testdata/config"}})

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
	})

	t.Run("should generate with default parser if not defined", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultFileParserId, "my_parser")
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":  flam.ConfigSourceDriverObservableFile,
				"disk_id": "my_disk",
				"path":    "/testdata/config"}})

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
	})
}
