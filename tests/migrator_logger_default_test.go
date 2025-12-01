package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultMigratorLogger_LogUpStart(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' up action started")
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

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerStartLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "migration '1.0.0' up action started")
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

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerStartLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"start": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "migration '1.0.0' up action started")
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
}

func Test_DefaultMigratorLogger_LogUpError(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "migration '1.0.0' up action error: error")
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

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerErrorLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "migration '1.0.0' up action error: error")
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

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerErrorLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"error": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "migration '1.0.0' up action error: error")
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
}

func Test_DefaultMigratorLogger_LogUpDone(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' up action terminated")
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

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerDoneLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "migration '1.0.0' up action terminated")
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

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerDoneLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"done": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "migration '1.0.0' up action terminated")
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

func Test_DefaultMigratorLogger_LogDownStart(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action started")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownStart(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerStartLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "migration '1.0.0' down action started")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownStart(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerStartLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"start": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "migration '1.0.0' down action started")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownStart(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})
}

func Test_DefaultMigratorLogger_LogDownError(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "migration '1.0.0' down action error: error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownError(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			}, errors.New("error"))
		}))
	})

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerErrorLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "migration '1.0.0' down action error: error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownError(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			}, errors.New("error"))
		}))
	})

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerErrorLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"error": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "migration '1.0.0' down action error: error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownError(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			}, errors.New("error"))
		}))
	})
}

func Test_DefaultMigratorLogger_LogDownDone(t *testing.T) {
	t.Run("should log on default channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownDone(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})

	t.Run("should log on assigned defaults channel and level if none is in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerDoneLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogWarning, "new-channel", "migration '1.0.0' down action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownDone(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})

	t.Run("should log on assigned given channel and level", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerChannel, "new-channel")
		_ = config.Set(flam.PathMigratorDefaultLoggerDoneLevel, flam.LogWarning)
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "my-channel",
				"levels": flam.Bag{
					"done": flam.LogFatal}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogFatal, "my-channel", "migration '1.0.0' down action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			logger, e := factory.Get("my_logger")
			require.NotNil(t, logger)
			require.NoError(t, e)

			logger.LogDownDone(flam.MigrationInfo{
				Version:     "1.0.0",
				Description: "description",
			})
		}))
	})
}
