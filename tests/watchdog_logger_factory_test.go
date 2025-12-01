package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_WatchdogLoggerFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added loggers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerAlphaMock := mocks.NewMockWatchdogLogger(ctrl)
		watchdogLoggerZuluMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			require.NoError(t, factory.Store("alpha", watchdogLoggerAlphaMock))
			require.NoError(t, factory.Store("zulu", watchdogLoggerZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added loggers and config defined loggers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		watchdogLoggerCharlieMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			require.NoError(t, factory.Store("charlie", watchdogLoggerCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_WatchdogLoggerFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated loggers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault},
			"alpha": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added loggers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerMock1 := mocks.NewMockWatchdogLogger(ctrl)
		watchdogLoggerMock2 := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			require.NoError(t, factory.Store("my_logger_1", watchdogLoggerMock1))
			require.NoError(t, factory.Store("my_logger_2", watchdogLoggerMock2))

			assert.Equal(t, []string{"my_logger_1", "my_logger_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated loggers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault},
			"alpha": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		watchdogLoggerMock1 := mocks.NewMockWatchdogLogger(ctrl)
		watchdogLoggerMock2 := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_logger_1", watchdogLoggerMock1))
			require.NoError(t, factory.Store("my_logger_2", watchdogLoggerMock2))

			assert.Equal(t, []string{"alpha", "my_logger_1", "my_logger_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_WatchdogLoggerFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
		"ny_logger_1": flam.Bag{}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	watchdogLoggerMock := mocks.NewMockWatchdogLogger(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
		require.NoError(t, factory.Store("ny_logger_2", watchdogLoggerMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_logger_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_logger_2",
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

func Test_WatchdogLoggerFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			got, e := factory.Get("my_logger")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved logger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			got, e := factory.Get("my_logger")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_logger")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_WatchdogLoggerFactory_Store(t *testing.T) {
	t.Run("should return nil reference if logger is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.ErrorIs(t, factory.Store("my_logger", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if logger reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		watchdogLoggerMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.ErrorIs(t, factory.Store("my_logger", watchdogLoggerMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if logger has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.NoError(t, factory.Store("my_logger", watchdogLoggerMock))
		}))
	})

	t.Run("should return duplicate resource if logger has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.NoError(t, factory.Store("my_logger", watchdogLoggerMock))
			assert.ErrorIs(t, factory.Store("my_logger", watchdogLoggerMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_WatchdogLoggerFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the logger is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.ErrorIs(t, factory.Remove("my_logger"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove logger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			require.NoError(t, factory.Store("my_logger", watchdogLoggerMock))

			assert.NoError(t, factory.Remove("my_logger"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_WatchdogLoggerFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored loggers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerMock1 := mocks.NewMockWatchdogLogger(ctrl)
		watchdogLoggerMock2 := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			require.NoError(t, factory.Store("my_logger_1", watchdogLoggerMock1))
			require.NoError(t, factory.Store("my_logger_2", watchdogLoggerMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
