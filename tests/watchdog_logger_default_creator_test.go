package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultWatchdogLoggerCreator(t *testing.T) {
	t.Run("should ignore config without/empty channel field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.Nil(t, logger)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should generate with default channel if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerChannel, "my_channel")
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "my_channel", "process [process_id] starting ...")
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

	t.Run("should generate with default start message log level if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerStartLevel, flam.LogNotice)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogNotice, "flam", "process [process_id] starting ...")
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

	t.Run("should generate with default error message log level if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerErrorLevel, flam.LogNotice)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogNotice, "flam", "process [process_id] error : error")
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

	t.Run("should generate with default done message log level if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogDefaultLoggerDoneLevel, flam.LogNotice)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.WatchdogLoggerDriverDefault,
				"channel": "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogNotice, "flam", "process [process_id] terminated")
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
