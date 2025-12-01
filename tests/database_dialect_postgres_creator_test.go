package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"

	"github.com/cjdias/flam-in-go"
)

func Test_PostgresDatabaseDialectCreator(t *testing.T) {
	t.Run("should ignore config without/empty username field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "",
				"password": "root",
				"schema":   "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty password field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "root",
				"password": "",
				"schema":   "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty host field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "root",
				"password": "root",
				"host":     "",
				"schema":   "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty port field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "root",
				"password": "root",
				"port":     0,
				"schema":   "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty schema field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "root",
				"password": "root",
				"schema":   ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should create with default host/port if none is given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "root",
				"password": "root",
				"schema":   "flam"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.NotNil(t, got)
			require.NoError(t, e)
			require.IsType(t, &postgres.Dialector{}, got)

			assert.Equal(t, "user=root password=root host=127.0.0.1 port=5432 dbname=flam", got.(*postgres.Dialector).DSN)
		}))
	})

	t.Run("should create with given host/port and extra params", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"host":     "192.168.100.100",
				"port":     5000,
				"username": "root",
				"password": "root",
				"schema":   "flam",
				"params": flam.Bag{
					"param1": "value1",
					"param2": "value2",
					"param3": "value3"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.NotNil(t, got)
			require.NoError(t, e)
			require.IsType(t, &postgres.Dialector{}, got)

			assert.Regexp(t, `^user\=root password\=root host\=192\.168\.100\.100 port\=5000 dbname\=flam`, got.(*postgres.Dialector).DSN)
			assert.Regexp(t, ` param1\=value1`, got.(*postgres.Dialector).DSN)
			assert.Regexp(t, ` param2\=value2`, got.(*postgres.Dialector).DSN)
			assert.Regexp(t, ` param3\=value3`, got.(*postgres.Dialector).DSN)
		}))
	})
}
