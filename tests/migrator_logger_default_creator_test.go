package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultMigrationLoggerCreator(t *testing.T) {
	t.Run("should ignore config without/empty channel field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.Nil(t, logger)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should generate with default channel if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "my_channel")
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "my_channel", "migration '1.0.0' up action started")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogUpStart(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})

	t.Run("should generate with default start message log level if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerStartLevel, flam.LogNotice)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogNotice, "flam", "migration '1.0.0' up action started")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogUpStart(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})

	t.Run("should generate with default error message log level if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerErrorLevel, flam.LogNotice)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogNotice, "flam", "migration '1.0.0' up action error: error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogUpError(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			}, errors.New("error"))
		}))
	})

	t.Run("should generate with default done message log level if none is given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerDoneLevel, flam.LogNotice)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogNotice, "flam", "migration '1.0.0' up action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogUpDone(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})
}
