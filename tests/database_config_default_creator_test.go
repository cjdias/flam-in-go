package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_DefaultDatabaseConfigCreator(t *testing.T) {
	t.Run("should return an error if logger.slow_threshold is not a duration", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault,
				"logger": flam.Bag{
					"slow_threshold": "invalid"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorContains(t, e, `time: invalid duration "invalid"`)
		}))
	})

	t.Run("should return an error if logger.level is not a valid log level", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault,
				"logger": flam.Bag{
					"level": "invalid"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownDatabaseLogLevel)
		}))
	})

	t.Run("should return an error if logger.type is not a valid log type", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault,
				"logger": flam.Bag{
					"type": "invalid"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownDatabaseLogType)
		}))
	})

	t.Run("should return an error if prepare_stmt_ttl is not a duration", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"default": flam.Bag{
				"driver":           flam.DatabaseConfigDriverDefault,
				"prepare_stmt_ttl": "invalid"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("default")
			assert.Nil(t, got)
			assert.ErrorContains(t, e, `time: invalid duration "invalid"`)
		}))
	})

	t.Run("should create with default values if no value is given", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"default": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("default")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.False(t, got.SkipDefaultTransaction)
			assert.False(t, got.FullSaveAssociations)
			assert.False(t, got.DryRun)
			assert.False(t, got.PrepareStmt)
			assert.Equal(t, 0, got.PrepareStmtMaxSize)
			assert.Equal(t, time.Duration(0), got.PrepareStmtTTL)
			assert.False(t, got.DisableAutomaticPing)
			assert.False(t, got.DisableForeignKeyConstraintWhenMigrating)
			assert.False(t, got.IgnoreRelationshipsWhenMigrating)
			assert.False(t, got.DisableNestedTransaction)
			assert.False(t, got.AllowGlobalUpdate)
			assert.False(t, got.QueryFields)
			assert.Equal(t, 0, got.CreateBatchSize)
			assert.False(t, got.TranslateError)
			assert.False(t, got.PropagateUnscoped)
		}))
	})

	t.Run("should create with given values", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"default": flam.Bag{
				"driver":                   flam.DatabaseConfigDriverDefault,
				"skip_default_transaction": true,
				"full_save_associations":   true,
				"dry_run":                  true,
				"prepare_stmt":             true,
				"prepare_stmt_max_size":    1,
				"prepare_stmt_ttl":         "1s",
				"disable_automatic_ping":   true,
				"disable_foreign_key_constraint_when_migrating": true,
				"ignore_relationships_when_migrating":           true,
				"disable_nested_transaction":                    true,
				"allow_global_update":                           true,
				"query_fields":                                  true,
				"create_batch_size":                             1,
				"translate_error":                               true,
				"propagate_unscoped":                            true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("default")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.True(t, got.SkipDefaultTransaction)
			assert.True(t, got.FullSaveAssociations)
			assert.True(t, got.DryRun)
			assert.True(t, got.PrepareStmt)
			assert.Equal(t, 1, got.PrepareStmtMaxSize)
			assert.Equal(t, time.Second, got.PrepareStmtTTL)
			assert.True(t, got.DisableAutomaticPing)
			assert.True(t, got.DisableForeignKeyConstraintWhenMigrating)
			assert.True(t, got.IgnoreRelationshipsWhenMigrating)
			assert.True(t, got.DisableNestedTransaction)
			assert.True(t, got.AllowGlobalUpdate)
			assert.True(t, got.QueryFields)
			assert.Equal(t, 1, got.CreateBatchSize)
			assert.True(t, got.TranslateError)
			assert.True(t, got.PropagateUnscoped)
		}))
	})

	t.Run("should allow valid logger configs", func(t *testing.T) {
		scenarios := []struct {
			name string
			cfg  flam.Bag
		}{
			{
				name: "default values",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault}},
			},
			{
				name: "with slow_threshold",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"slow_threshold": "1s"}}},
			},
			{
				name: "silent log level",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"type":  flam.DatabaseConfigLoggerDefault,
							"level": "silent"}}},
			},
			{
				name: "error log level",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"type":  flam.DatabaseConfigLoggerDefault,
							"level": "error"}}},
			},
			{
				name: "warn log level",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"type":  flam.DatabaseConfigLoggerDefault,
							"level": "warn"}}},
			},
			{
				name: "info log level",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"type":  flam.DatabaseConfigLoggerDefault,
							"level": "info"}}},
			},
			{
				name: "discard log type",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"type": flam.DatabaseConfigLoggerDiscard}}},
			},
			{
				name: "default log type",
				cfg: flam.Bag{
					"default": flam.Bag{
						"driver": flam.DatabaseConfigDriverDefault,
						"logger": flam.Bag{
							"type": flam.DatabaseConfigLoggerDefault}}},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				config := flam.Bag{}
				_ = config.Set(flam.PathDatabaseConfigs, scenario.cfg)

				app := flam.NewApplication(config)
				defer func() { _ = app.Close() }()

				require.NoError(t, app.Boot())

				assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
					got, e := factory.Get("default")
					assert.NotNil(t, got)
					assert.NoError(t, e)
				}))
			})
		}
	})
}
