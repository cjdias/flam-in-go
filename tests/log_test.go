package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_Logger_Signal(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogInfo, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.Signal(flam.LogInfo, "channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogInfo, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.Signal(flam.LogInfo, "channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogInfo, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.Signal(flam.LogInfo, "channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_SignalFatal(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogFatal, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalFatal("channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogFatal, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalFatal("channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogFatal, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalFatal("channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_SignalError(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogError, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalError("channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogError, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalError("channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogError, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalError("channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_SignalWarning(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogWarning, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalWarning("channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogWarning, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalWarning("channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogWarning, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalWarning("channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_SignalNotice(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogNotice, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalNotice("channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogNotice, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalNotice("channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogNotice, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalNotice("channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_SignalInfo(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogInfo, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalInfo("channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogInfo, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalInfo("channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogInfo, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalInfo("channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_SignalDebug(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogDebug, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalDebug("channel", "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogDebug, "channel", "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalDebug("channel", "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogDebug, "channel", "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalDebug("channel", "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_Broadcast(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.Broadcast(flam.LogInfo, "message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.Broadcast(flam.LogInfo, "message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.Broadcast(flam.LogInfo, "message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_BroadcastFatal(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogFatal, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastFatal("message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogFatal, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastFatal("message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogFatal, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastFatal("message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_BroadcastError(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogError, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastError("message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogError, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastError("message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogError, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastError("message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_BroadcastWarning(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogWarning, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastWarning("message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogWarning, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastWarning("message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogWarning, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastWarning("message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_BroadcastNotice(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogNotice, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastNotice("message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogNotice, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastNotice("message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogNotice, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastNotice("message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_BroadcastInfo(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastInfo("message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastInfo("message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastInfo("message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_BroadcastDebug(t *testing.T) {
	t.Run("should not send message to stream if not flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogDebug, "message", flam.Bag{}).
			Return(nil).
			Times(0)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastDebug("message")
		}))
	})

	t.Run("should send message to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogDebug, "message", flam.Bag{}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastDebug("message")

			assert.NoError(t, log.Flush())
		}))
	})

	t.Run("should send message with context to stream if flushed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogDebug, "message", flam.Bag{"key": "value"}).
			Return(nil).
			Times(1)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastDebug("message", flam.Bag{"key": "value"})

			assert.NoError(t, log.Flush())
		}))
	})
}

func Test_Logger_Flush(t *testing.T) {
	t.Run("should return the signal message forward action to stream error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		expectedErr := errors.New("expected error")
		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Signal(gomock.Any(), flam.LogNotice, "channel", "message3", flam.Bag{"key3": "value3"}).
			Return(expectedErr)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.SignalNotice("channel", "message3", flam.Bag{"key3": "value3"})

			assert.ErrorIs(t, log.Flush(), expectedErr)
		}))
	})

	t.Run("should return the broadcast message forward action to stream error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		expectedErr := errors.New("expected error")
		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogNotice, "message3", flam.Bag{"key3": "value3"}).
			Return(expectedErr)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastNotice("message3", flam.Bag{"key3": "value3"})

			assert.ErrorIs(t, log.Flush(), expectedErr)
		}))
	})

	t.Run("should send stored messages to stream", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		streamMock := mocks.NewMockLogStream(ctrl)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogDebug, "message1", flam.Bag{"key1": "value1"}).
			Return(nil)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogInfo, "message2", flam.Bag{"key2": "value2"}).
			Return(nil)
		streamMock.EXPECT().
			Broadcast(gomock.Any(), flam.LogNotice, "message3", flam.Bag{"key3": "value3"}).
			Return(nil)
		streamMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory, log flam.Logger) {
			require.NoError(t, factory.Store("stream", streamMock))

			log.BroadcastDebug("message1", flam.Bag{"key1": "value1"})
			log.BroadcastInfo("message2", flam.Bag{"key2": "value2"})
			log.BroadcastNotice("message3", flam.Bag{"key3": "value3"})

			assert.NoError(t, log.Flush())
		}))
	})
}
