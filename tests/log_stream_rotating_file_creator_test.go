package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_RotatingFileLogStreamCreator(t *testing.T) {
	t.Run("should ignore config without/empty serializer_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "",
				"disk_id":       "my_disk",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty disk_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty path field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should return serialization creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should return disk creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should generate with default log level if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultLevel, "notice")
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file-%s"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		diskMock := afero.NewMemMapFs()
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			stream, e := factory.Get("my_stream")
			assert.NotNil(t, stream)
			assert.NoError(t, e)

			assert.Equal(t, flam.LogNotice, stream.GetLevel())
		}))
	})

	t.Run("should generate with default serializer if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultSerializerId, "my_serializer")
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":  flam.LogStreamDriverRotatingFile,
				"disk_id": "my_disk",
				"path":    "/file-%s"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		diskMock := afero.NewMemMapFs()
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())
	})

	t.Run("should generate with default disk if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultDiskId, "my_disk")
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"path":          "/file-%s"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		diskMock := afero.NewMemMapFs()
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())
	})
}
