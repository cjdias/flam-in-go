package tests

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_FileLogStream(t *testing.T) {
	t.Run("should correctly handle the stream level", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
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
				"path":          "/file",
				"level":         "debug"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			got, e := factory.Get("my_stream")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Equal(t, flam.LogDebug, got.GetLevel())

			assert.NoError(t, got.SetLevel(flam.LogInfo))
			assert.Equal(t, flam.LogInfo, got.GetLevel())
		}))
	})

	t.Run("should correctly handle the channel list", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
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
				"path":          "/file",
				"level":         "debug",
				"channels":      []any{"channel_2", "channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			got, e := factory.Get("my_stream")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.True(t, got.HasChannel("channel_1"))
			assert.True(t, got.HasChannel("channel_2"))
			assert.False(t, got.HasChannel("channel_3"))
			assert.ElementsMatch(t, got.ListChannels(), []string{"channel_1", "channel_2"})

			require.NoError(t, got.AddChannel("channel_3"))
			assert.True(t, got.HasChannel("channel_1"))
			assert.True(t, got.HasChannel("channel_2"))
			assert.True(t, got.HasChannel("channel_3"))
			assert.ElementsMatch(t, got.ListChannels(), []string{"channel_1", "channel_2", "channel_3"})

			require.NoError(t, got.RemoveChannel("channel_2"))
			assert.True(t, got.HasChannel("channel_1"))
			assert.False(t, got.HasChannel("channel_2"))
			assert.True(t, got.HasChannel("channel_3"))
			assert.ElementsMatch(t, got.ListChannels(), []string{"channel_1", "channel_3"})

			require.NoError(t, got.RemoveAllChannels())
			assert.False(t, got.HasChannel("channel_1"))
			assert.False(t, got.HasChannel("channel_2"))
			assert.False(t, got.HasChannel("channel_3"))
			assert.ElementsMatch(t, got.ListChannels(), []string{})
		}))
	})

	t.Run("should correctly handle the stream signal", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
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
				"path":          "/file",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory, log flam.Logger) {
			log.Signal(flam.LogDebug, "channel_1", "channel 1 : debug message")
			log.Signal(flam.LogFatal, "channel_2", "channel 2 : fatal message")
			log.Signal(flam.LogFatal, "channel_1", "channel 1 : fatal message")

			assert.NoError(t, log.Flush())

			disk, e := factory.Get("my_disk")
			require.NotNil(t, disk)
			require.NoError(t, e)

			file, _ := disk.OpenFile("/file", os.O_RDONLY, os.FileMode(0o644))
			data, _ := io.ReadAll(file)
			stringData := string(data)

			rx := `^`
			rx += `{\s*`
			rx += `"channel"\s*\:\s*"channel_1",\s*`
			rx += `"level"\s*\:\s*"FATAL",\s*`
			rx += `"message"\s*\:\s*"channel 1 : fatal message",\s*`
			rx += `"time"\s*\:\s*"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}"\s*`
			rx += `}\s*`
			rx += `$`
			assert.Regexp(t, rx, stringData)
		}))
	})

	t.Run("should correctly handle the stream signal (any channel)", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
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
				"path":          "/file",
				"level":         "warning",
				"channels":      []any{"*"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory, log flam.Logger) {
			log.Signal(flam.LogDebug, "channel_1", "channel 1 : debug message")
			log.Signal(flam.LogFatal, "channel_2", "channel 2 : fatal message")
			log.Signal(flam.LogFatal, "channel_1", "channel 1 : fatal message")

			assert.NoError(t, log.Flush())

			disk, e := factory.Get("my_disk")
			require.NotNil(t, disk)
			require.NoError(t, e)

			file, _ := disk.OpenFile("/file", os.O_RDONLY, os.FileMode(0o644))
			data, _ := io.ReadAll(file)
			stringData := string(data)

			rx := `^`
			rx += `{\s*`
			rx += `"channel"\s*\:\s*"channel_2",\s*`
			rx += `"level"\s*\:\s*"FATAL",\s*`
			rx += `"message"\s*\:\s*"channel 2 : fatal message",\s*`
			rx += `"time"\s*\:\s*"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}"\s*`
			rx += `}\s*`
			rx += `{\s*`
			rx += `"channel"\s*\:\s*"channel_1",\s*`
			rx += `"level"\s*\:\s*"FATAL",\s*`
			rx += `"message"\s*\:\s*"channel 1 : fatal message",\s*`
			rx += `"time"\s*\:\s*"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}"\s*`
			rx += `}\s*`
			rx += `$`
			assert.Regexp(t, rx, stringData)
		}))
	})

	t.Run("should correctly handle the stream broadcast", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
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
				"path":          "/file",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory, log flam.Logger) {
			log.Broadcast(flam.LogDebug, "channel 1 : debug message")
			log.Broadcast(flam.LogFatal, "channel 2 : fatal message")
			log.Broadcast(flam.LogFatal, "channel 1 : fatal message")

			assert.NoError(t, log.Flush())

			disk, e := factory.Get("my_disk")
			require.NotNil(t, disk)
			require.NoError(t, e)

			file, _ := disk.OpenFile("/file", os.O_RDONLY, os.FileMode(0o644))
			data, _ := io.ReadAll(file)
			stringData := string(data)

			rx := `^`
			rx += `{\s*`
			rx += `"level"\s*\:\s*"FATAL",\s*`
			rx += `"message"\s*\:\s*"channel 2 : fatal message",\s*`
			rx += `"time"\s*\:\s*"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}"\s*`
			rx += `}\s*`
			rx += `{\s*`
			rx += `"level"\s*\:\s*"FATAL",\s*`
			rx += `"message"\s*\:\s*"channel 1 : fatal message",\s*`
			rx += `"time"\s*\:\s*"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}"\s*`
			rx += `}\s*`
			rx += `$`
			assert.Regexp(t, rx, stringData)
		}))
	})

	t.Run("should return the stream writer closing error", func(t *testing.T) {
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
				"path":          "/file",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})
		app := flam.NewApplication(config)

		expectedError := errors.New("expected error")
		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Close().Return(expectedError)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().
			OpenFile("/file", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(fileMock, nil).
			Times(1)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			require.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.ErrorIs(t, app.Close(), expectedError)
	})
}
