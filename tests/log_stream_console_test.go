package tests

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_ConsoleLogStream(t *testing.T) {
	t.Run("should correctly handle the stream level", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer",
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
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer",
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
		old := os.Stdout // keep backup of the real stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			outC <- buf.String()
		}()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Signal(flam.LogDebug, "channel_1", "channel 1 : debug message")
			log.Signal(flam.LogFatal, "channel_2", "channel 2 : fatal message")
			log.Signal(flam.LogFatal, "channel_1", "channel 1 : fatal message")

			assert.NoError(t, log.Flush())
		}))

		_ = w.Close()
		os.Stdout = old // restoring the real stdout
		out := <-outC

		rx := `^`
		rx += `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}\s*`
		rx += `\[FATAL\]\s*`
		rx += `channel 1 : fatal message\s*`
		rx += `$`
		assert.Regexp(t, rx, out)
	})

	t.Run("should correctly handle the stream signal (any channel)", func(t *testing.T) {
		old := os.Stdout // keep backup of the real stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			outC <- buf.String()
		}()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer",
				"level":         "warning",
				"channels":      []any{"*"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Signal(flam.LogDebug, "channel_1", "channel 1 : debug message")
			log.Signal(flam.LogFatal, "channel_2", "channel 2 : fatal message")
			log.Signal(flam.LogFatal, "channel_1", "channel 1 : fatal message")

			assert.NoError(t, log.Flush())
		}))

		_ = w.Close()
		os.Stdout = old // restoring the real stdout
		out := <-outC

		rx := `^`
		rx += `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}\s*`
		rx += `\[FATAL\]\s*`
		rx += `channel 2 : fatal message\s*`
		rx += `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}\s*`
		rx += `\[FATAL\]\s*`
		rx += `channel 1 : fatal message\s*`
		rx += `$`
		assert.Regexp(t, rx, out)
	})

	t.Run("should correctly handle the broadcast signal", func(t *testing.T) {
		old := os.Stdout // keep backup of the real stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			outC <- buf.String()
		}()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer",
				"level":         "warning",
				"channels":      []any{"channel_1"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(log flam.Logger) {
			log.Broadcast(flam.LogDebug, "channel 1 : debug message")
			log.Broadcast(flam.LogFatal, "channel 2 : fatal message")
			log.Broadcast(flam.LogFatal, "channel 1 : fatal message")

			assert.NoError(t, log.Flush())
		}))

		_ = w.Close()
		os.Stdout = old // restoring the real stdout
		out := <-outC

		rx := `^`
		rx += `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}\s*`
		rx += `\[FATAL\]\s*`
		rx += `channel 2 : fatal message\s*`
		rx += `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+\d{4}\s*`
		rx += `\[FATAL\]\s*`
		rx += `channel 1 : fatal message\s*`
		rx += `$`
		assert.Regexp(t, rx, out)
	})
}
