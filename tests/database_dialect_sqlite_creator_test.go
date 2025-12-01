package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"

	"github.com/cjdias/flam-in-go"
)

func Test_SqliteDatabaseDialectCreator(t *testing.T) {
	t.Run("should ignore config without/empty host field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite,
				"host":   ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.Nil(t, got)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should create with default host if none is given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("default")
			require.NotNil(t, got)
			require.NoError(t, e)
			require.IsType(t, &sqlite.Dialector{}, got)

			assert.Equal(t, ":memory:", got.(*sqlite.Dialector).DSN)
		}))
	})

	t.Run("should create with given host and extra params", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite,
				"host":   "192.168.1.1",
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
			require.IsType(t, &sqlite.Dialector{}, got)

			assert.Regexp(t, `^192\.168\.1\.1\?`, got.(*sqlite.Dialector).DSN)
			assert.Regexp(t, `\&param1\=value1`, got.(*sqlite.Dialector).DSN)
			assert.Regexp(t, `\&param2\=value2`, got.(*sqlite.Dialector).DSN)
			assert.Regexp(t, `\&param3\=value3`, got.(*sqlite.Dialector).DSN)
		}))
	})
}
