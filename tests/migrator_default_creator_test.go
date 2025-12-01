package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultMigratorCreator(t *testing.T) {
	t.Run("should ignore config without/empty connection_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "",
				"logger_id":     "my_logger",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.Nil(t, migrator)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty group field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.Nil(t, migrator)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return connection creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			connection, e := factory.Get("my_migrator")
			require.Nil(t, connection)
			require.Error(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return logger creation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			connection, e := factory.Get("my_migrator")
			require.Nil(t, connection)
			require.Error(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should generate with default connection if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultConnectionId, "my_connection")
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":    flam.MigratorDriverDefault,
				"logger_id": "my_logger",
				"group":     "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			connection, e := factory.Get("my_migrator")
			require.NotNil(t, connection)
			require.NoError(t, e)
		}))
	})

	t.Run("should generate with default logger if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerId, "my_logger")
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			connection, e := factory.Get("my_migrator")
			require.NotNil(t, connection)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate a migrator without a logger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerId, "")
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			connection, e := factory.Get("my_migrator")
			require.NotNil(t, connection)
			require.NoError(t, e)
		}))
	})

	t.Run("should return auto migration error on creation if occurs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorDefaultLoggerId, "")
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("auto migrate error")
		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			connection, e := factory.Get("my_migrator")
			require.Nil(t, connection)
			require.ErrorIs(t, e, expectedErr)
		}))
	})
}
