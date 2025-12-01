package tests

import (
	"io"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_LogStreamFactory_Close(t *testing.T) {
	t.Run("should correctly close stored streams", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.NoError(t, factory.Store("my_stream", logStreamMock))
		}))

		assert.NoError(t, app.Close())
	})
}

func Test_LogStreamFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added streams", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamAlphaMock := mocks.NewMockLogStream(ctrl)
		logStreamAlphaMock.EXPECT().Close().Return(nil)

		logStreamZuluMock := mocks.NewMockLogStream(ctrl)
		logStreamZuluMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			require.NoError(t, factory.Store("alpha", logStreamAlphaMock))
			require.NoError(t, factory.Store("zulu", logStreamZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added streams and config defined streams", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		logStreamCharlieMock := mocks.NewMockLogStream(ctrl)
		logStreamCharlieMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			require.NoError(t, factory.Store("charlie", logStreamCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_LogStreamFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated streams", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"zulu": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"},
			"alpha": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added streams", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamMock1 := mocks.NewMockLogStream(ctrl)
		logStreamMock1.EXPECT().Close().Return(nil)

		logStreamMock2 := mocks.NewMockLogStream(ctrl)
		logStreamMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			require.NoError(t, factory.Store("my_stream_1", logStreamMock1))
			require.NoError(t, factory.Store("my_stream_2", logStreamMock2))

			assert.Equal(t, []string{"my_stream_1", "my_stream_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated streams", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"zulu": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"},
			"alpha": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		logStreamMock1 := mocks.NewMockLogStream(ctrl)
		logStreamMock1.EXPECT().Close().Return(nil)

		logStreamMock2 := mocks.NewMockLogStream(ctrl)
		logStreamMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_stream_1", logStreamMock1))
			require.NoError(t, factory.Store("my_stream_2", logStreamMock2))

			assert.Equal(t, []string{"alpha", "my_stream_1", "my_stream_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_LogStreamFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathLogStreams, flam.Bag{
		"ny_stream_1": flam.Bag{
			"driver": flam.LogStreamDriverConsole}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	logStreamMock := mocks.NewMockLogStream(ctrl)
	logStreamMock.EXPECT().Close().Return(nil)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
		require.NoError(t, factory.Store("ny_stream_2", logStreamMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_stream_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_stream_2",
				expected: true},
			{
				name:     "non-existent entry",
				id:       "nonexistent",
				expected: false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, factory.Has(tc.id))
			})
		}
	}))
}

func Test_LogStreamFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			got, e := factory.Get("my_stream")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved stream", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			got, e := factory.Get("my_stream")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_stream")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})

	t.Run("should add the generated stream to the logger", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverFile,
				"disk_id":       "my_disk",
				"serializer_id": "my_serializer",
				"path":          "/my_stream.log",
				"level":         "debug"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, logger flam.Logger) {
			got, e := factory.Get("my_stream")
			require.NotNil(t, got)
			require.NoError(t, e)

			logger.BroadcastFatal("my message")
			require.NoError(t, logger.Flush())
		}))

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk, e := factory.Get("my_disk")
			require.NoError(t, e)
			require.NotNil(t, disk)

			file, e := disk.Open("/my_stream.log")
			require.NoError(t, e)
			require.NotNil(t, file)
			defer func() { _ = file.Close() }()

			b, e := io.ReadAll(file)
			require.NoError(t, e)

			assert.Contains(t, string(b), "[FATAL] my message")
		}))
	})
}

func Test_LogStreamFactory_Store(t *testing.T) {
	t.Run("should return nil reference if stream is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.ErrorIs(t, factory.Store("my_stream", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if stream reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver": flam.LogStreamDriverConsole}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		logStreamMock := mocks.NewMockLogStream(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.ErrorIs(t, factory.Store("my_stream", logStreamMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if stream has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.NoError(t, factory.Store("my_stream", logStreamMock))
		}))
	})

	t.Run("should return duplicate resource if stream has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.NoError(t, factory.Store("my_stream", logStreamMock))
			assert.ErrorIs(t, factory.Store("my_stream", logStreamMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should add the stored stream to the logger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Broadcast(gomock.Any(), flam.LogFatal, "my message", flam.Bag{})
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, logger flam.Logger) {
			e := factory.Store("my_stream", logStreamMock)
			require.NoError(t, e)

			logger.BroadcastFatal("my message")
			require.NoError(t, logger.Flush())
		}))
	})
}

func Test_LogStreamFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the stream is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.ErrorIs(t, factory.Remove("my_stream"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove stream", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			require.NoError(t, factory.Store("my_stream", logStreamMock))

			assert.NoError(t, factory.Remove("my_stream"))

			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should remove the stream from the logger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, logger flam.Logger) {
			e := factory.Store("my_stream", logStreamMock)
			require.NoError(t, e)

			assert.NoError(t, factory.Remove("my_stream"))

			logger.BroadcastFatal("my message")
			assert.NoError(t, logger.Flush())
		}))
	})
}

func Test_LogStreamFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored streams", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamMock1 := mocks.NewMockLogStream(ctrl)
		logStreamMock1.EXPECT().Close().Return(nil)

		logStreamMock2 := mocks.NewMockLogStream(ctrl)
		logStreamMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			require.NoError(t, factory.Store("my_stream_1", logStreamMock1))
			require.NoError(t, factory.Store("my_stream_2", logStreamMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should remove all the streams from the logger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, logger flam.Logger) {
			e := factory.Store("my_stream", logStreamMock)
			require.NoError(t, e)

			assert.NoError(t, factory.RemoveAll())

			logger.BroadcastFatal("my message")
			assert.NoError(t, logger.Flush())
		}))
	})
}
