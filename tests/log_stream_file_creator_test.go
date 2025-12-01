package tests

import (
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_FileLogStreamCreator(t *testing.T) {
	t.Run("should ignore config without/empty serializer_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverFile,
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
				"driver":        flam.LogStreamDriverFile,
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
				"driver":        flam.LogStreamDriverFile,
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
				"driver":        flam.LogStreamDriverFile,
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
				"driver":        flam.LogStreamDriverFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should return file creation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedError := errors.New("expected error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().
			OpenFile("/file", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(nil, expectedError).
			Times(1)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			require.NoError(t, e)
		}))

		assert.ErrorIs(t, app.Boot(), expectedError)
	})

	t.Run("should generate with default level if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultLevel, "notice")
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			got, e := factory.Get("my_stream")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Equal(t, flam.LogNotice, got.GetLevel())
		}))
	})

	t.Run("should generate with default serializer if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultSerializerId, "my_serializer")
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":  flam.LogStreamDriverFile,
				"disk_id": "my_disk",
				"path":    "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())
	})

	t.Run("should generate with default disk if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultDiskId, "my_disk")
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverFile,
				"serializer_id": "my_serializer",
				"path":          "/file"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())
	})
}
