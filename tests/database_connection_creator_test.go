package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_DatabaseConnectionCreator(t *testing.T) {
	t.Run("should ignore config without/empty dialect_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"default": flam.Bag{
				"dialect_id": "",
				"config_id":  "my_config"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty config_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"default": flam.Bag{
				"dialect_id": "my_dialog",
				"config_id":  ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return dialect creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"default": flam.Bag{
				"dialect_id": "my_dialect",
				"config_id":  "my_config"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"default": flam.Bag{
				"dialect_id": "my_dialect",
				"config_id":  "my_config"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return the created connection", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"default": flam.Bag{
				"dialect_id": "my_dialect",
				"config_id":  "my_config"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			got, e := factory.Get("default")
			assert.NotNil(t, got)
			assert.NoError(t, e)
		}))
	})

	t.Run("should fallback to default dialect and config if not given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDefaultDialectId, "my_dialect")
		_ = config.Set(flam.PathDatabaseDefaultConfigId, "my_config")
		_ = config.Set(flam.PathConfigSources, flam.Bag{})
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"default": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			got, e := factory.Get("default")
			assert.NotNil(t, got)
			assert.NoError(t, e)
		}))
	})
}
