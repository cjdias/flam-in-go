package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultWatchdogLogger_LogStart(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] starting ...")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogStart("process_id")
		}))
	})

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathWatchdogDefaultLoggerStartLevel, flam.LogWarning)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "process [process_id] starting ...")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogStart("process_id")
		}))
	})

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathWatchdogDefaultLoggerStartLevel, flam.LogWarning)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"start": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "process [process_id] starting ...")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogStart("process_id")
		}))
	})
}

func Test_DefaultWatchdogLogger_LogError(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "process [process_id] error : error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogError("process_id", errors.New("error"))
		}))
	})

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathWatchdogDefaultLoggerErrorLevel, flam.LogWarning)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "process [process_id] error : error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogError("process_id", errors.New("error"))
		}))
	})

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathWatchdogDefaultLoggerErrorLevel, flam.LogWarning)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"error": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "process [process_id] error : error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogError("process_id", errors.New("error"))
		}))
	})
}

func Test_DefaultWatchdogLogger_LogDone(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDone("process_id")
		}))
	})

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathWatchdogDefaultLoggerDoneLevel, flam.LogWarning)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "process [process_id] terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDone("process_id")
		}))
	})

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathWatchdogDefaultLoggerDoneLevel, flam.LogWarning)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"done": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "process [process_id] terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDone("process_id")
		}))
	})
}
