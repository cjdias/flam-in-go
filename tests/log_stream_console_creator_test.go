package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_ConsoleLogStreamCreator(t *testing.T) {
	t.Run("should ignore config without/empty serializer_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should return serialization creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer",
				"level":         "debug"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should generate with the default log level if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultLevel, "notice")
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"}})

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

	t.Run("should generate with the default serializer if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogDefaultSerializerId, "my_serializer")
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver": flam.LogStreamDriverConsole}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())
	})
}
