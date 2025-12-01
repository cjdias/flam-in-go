package tests

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_RotatingFileLogStream(t *testing.T) {
	t.Run("should return file opening error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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
				"path":          "/file-%s"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedError := fmt.Errorf("error")
		now := time.Now()
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().
			OpenFile(fmt.Sprintf("/file-%s", now.Format("2006-01-02")), os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(nil, expectedError)
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		assert.ErrorIs(t, app.Boot(), expectedError)
	})

	t.Run("should correctly handle the stream level", func(t *testing.T) {
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
				"path":          "/file-%s",
				"level":         "debug"}})

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

			assert.Equal(t, flam.LogDebug, stream.GetLevel())

			assert.NoError(t, stream.SetLevel(flam.LogInfo))
			assert.Equal(t, flam.LogInfo, stream.GetLevel())
		}))
	})

	t.Run("should correctly handle the channel list", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file-%s",
				"level":         "debug",
				"channels":      []any{"channel_2", "channel_1"}}})

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
			require.NotNil(t, stream)
			require.NoError(t, e)

			assert.True(t, stream.HasChannel("channel_1"))
			assert.True(t, stream.HasChannel("channel_2"))
			assert.False(t, stream.HasChannel("channel_3"))
			assert.ElementsMatch(t, stream.ListChannels(), []string{"channel_1", "channel_2"})

			require.NoError(t, stream.AddChannel("channel_3"))
			assert.True(t, stream.HasChannel("channel_1"))
			assert.True(t, stream.HasChannel("channel_2"))
			assert.True(t, stream.HasChannel("channel_3"))
			assert.ElementsMatch(t, stream.ListChannels(), []string{"channel_1", "channel_2", "channel_3"})

			require.NoError(t, stream.RemoveChannel("channel_2"))
			assert.True(t, stream.HasChannel("channel_1"))
			assert.False(t, stream.HasChannel("channel_2"))
			assert.True(t, stream.HasChannel("channel_3"))
			assert.ElementsMatch(t, stream.ListChannels(), []string{"channel_1", "channel_3"})

			require.NoError(t, stream.RemoveAllChannels())
			assert.False(t, stream.HasChannel("channel_1"))
			assert.False(t, stream.HasChannel("channel_2"))
			assert.False(t, stream.HasChannel("channel_3"))
			assert.ElementsMatch(t, stream.ListChannels(), []string{})
		}))
	})

	t.Run("should correctly handle the stream signal", func(t *testing.T) {
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
				"path":          "/file-%s",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		diskMock := afero.NewMemMapFs()
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Signal(flam.LogDebug, "channel_1", "channel 1 : debug message")
			log.Signal(flam.LogFatal, "channel_2", "channel 2 : fatal message")
			log.Signal(flam.LogFatal, "channel_1", "channel 1 : fatal message")

			assert.NoError(t, log.Flush())
		}))

		now := time.Now()
		fileName := fmt.Sprintf("/file-%s", now.Format("2006-01-02"))
		file, _ := diskMock.OpenFile(fileName, os.O_RDONLY, os.FileMode(0o644))
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
	})

	t.Run("should correctly handle the stream signal (any channel)", func(t *testing.T) {
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
				"path":          "/file-%s",
				"level":         "warning",
				"channels":      []any{"*"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		diskMock := afero.NewMemMapFs()
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Signal(flam.LogDebug, "channel_1", "channel 1 : debug message")
			log.Signal(flam.LogFatal, "channel_2", "channel 2 : fatal message")
			log.Signal(flam.LogFatal, "channel_1", "channel 1 : fatal message")

			assert.NoError(t, log.Flush())
		}))

		now := time.Now()
		fileName := fmt.Sprintf("/file-%s", now.Format("2006-01-02"))
		file, _ := diskMock.OpenFile(fileName, os.O_RDONLY, os.FileMode(0o644))
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
	})

	t.Run("should correctly handle the stream broadcast", func(t *testing.T) {
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
				"path":          "/file-%s",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		diskMock := afero.NewMemMapFs()
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Broadcast(flam.LogDebug, "channel 1 : debug message")
			log.Broadcast(flam.LogFatal, "channel 2 : fatal message")
			log.Broadcast(flam.LogFatal, "channel 1 : fatal message")

			assert.NoError(t, log.Flush())
		}))

		now := time.Now()
		fileName := fmt.Sprintf("/file-%s", now.Format("2006-01-02"))
		file, _ := diskMock.OpenFile(fileName, os.O_RDONLY, os.FileMode(0o644))
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
				"driver":        flam.LogStreamDriverRotatingFile,
				"serializer_id": "my_serializer",
				"disk_id":       "my_disk",
				"path":          "/file-%s",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)

		expectedError := errors.New("expected error")
		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Close().Return(expectedError)

		now := time.Now()
		fileName := fmt.Sprintf("/file-%s", now.Format("2006-01-02"))
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().
			OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(fileMock, nil).
			Times(1)
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.ErrorIs(t, app.Close(), expectedError)
	})

	t.Run("should correctly rotate the file on date change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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
				"path":          "/file-%s",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		triggerMock := mocks.NewMockTrigger(ctrl)
		triggerMock.EXPECT().Close().Return(nil)

		triggerFactoryMock := mocks.NewMockTriggerFactory(ctrl)
		triggerFactoryMock.EXPECT().NewRecurring(gomock.Any(), gomock.Any()).Return(triggerMock, nil)
		require.NoError(t, app.Container().Decorate(func(flam.TriggerFactory) flam.TriggerFactory {
			return triggerFactoryMock
		}))

		timerMock := mocks.NewMockTimer(ctrl)
		gomock.InOrder(
			timerMock.EXPECT().Now().Return(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			timerMock.EXPECT().Now().Return(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)),
		)
		require.NoError(t, app.Container().Decorate(func(flam.Timer) flam.Timer {
			return timerMock
		}))

		file1Mock := mocks.NewMockFile(ctrl)
		file1Mock.EXPECT().Close().Return(nil)
		file2Mock := mocks.NewMockFile(ctrl)
		file2Mock.EXPECT().Write(gomock.Any()).Return(0, nil)
		file2Mock.EXPECT().Close().Return(nil)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().
			OpenFile("/file-2021-01-01", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(file1Mock, nil).
			Times(1)
		diskMock.EXPECT().
			OpenFile("/file-2021-01-02", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(file2Mock, nil).
			Times(1)
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Broadcast(flam.LogFatal, "debug message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should return the rotating file opening error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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
				"path":          "/file-%s",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)

		triggerMock := mocks.NewMockTrigger(ctrl)

		triggerFactoryMock := mocks.NewMockTriggerFactory(ctrl)
		triggerFactoryMock.EXPECT().NewRecurring(gomock.Any(), gomock.Any()).Return(triggerMock, nil)
		require.NoError(t, app.Container().Decorate(func(flam.TriggerFactory) flam.TriggerFactory {
			return triggerFactoryMock
		}))

		timerMock := mocks.NewMockTimer(ctrl)
		gomock.InOrder(
			timerMock.EXPECT().Now().Return(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			timerMock.EXPECT().Now().Return(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)),
		)
		require.NoError(t, app.Container().Decorate(func(flam.Timer) flam.Timer {
			return timerMock
		}))

		file1Mock := mocks.NewMockFile(ctrl)

		expectedError := errors.New("expected error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().
			OpenFile("/file-2021-01-01", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(file1Mock, nil).
			Times(1)
		diskMock.EXPECT().
			OpenFile("/file-2021-01-02", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0o644)).
			Return(nil, expectedError).
			Times(1)
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			e := factory.Store("my_disk", diskMock)
			assert.NoError(t, e)
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Broadcast(flam.LogFatal, "debug message")

			assert.ErrorIs(t, log.Flush(), expectedError)
		}))
	})
}
