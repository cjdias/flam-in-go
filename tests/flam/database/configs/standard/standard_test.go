package standard

import (
	"errors"
	"io"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// create returns the configuration "cfg" of the fields, created by the
// factory of an app with the standard bundle.
func create(t *testing.T, fields container.Bag) (flam.DatabaseConfig, error) {
	t.Helper()
	bag := container.Bag{}
	for _, b := range []flam.IBundle{config.NewBundle(), database.NewBundle(), standard.NewBundle()} {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = standard.Driver
	}
	testutil.NoError(t, bag.Set(database.ConfigConfigPath+".cfg", fields))

	app := testutil.NewApp()
	for _, b := range []flam.IBundle{config.NewBundle(), database.NewBundle(), standard.NewBundle()} {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var cfg flam.DatabaseConfig
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IDatabaseConfigFactory) {
		cfg, err = f.Get("cfg")
	}))
	return cfg, err
}

// loggerOf returns the configuration and the output of the gorm logger.
func loggerOf(t *testing.T, l logger.Interface) (logger.Config, io.Writer) {
	t.Helper()
	v := reflect.ValueOf(l).Elem()
	return v.FieldByName("Config").Interface().(logger.Config), v.FieldByName("Writer").Interface().(*log.Logger).Writer()
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "Id", standard.Id, "standard")
	testutil.Equal(t, "Driver", standard.Driver, "flam.database.configs.driver.standard")
	testutil.Equal(t, "LoggerDefault", standard.LoggerDefault, "default")
	testutil.Equal(t, "LoggerDiscard", standard.LoggerDiscard, "discard")
	testutil.Equal(t, "ConfigFieldLoggerLevel", standard.ConfigFieldLoggerLevel, "logger.level")
}

func Test_Bundle(t *testing.T) {
	t.Run("should configure the standard configuration as the default", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.database.configs.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "configs", bag.Get(database.ConfigConfigPath), any(container.Bag{
			standard.Id: container.Bag{
				factory.ConfigFieldDriver:  standard.Driver,
				standard.ConfigFieldLogger: container.Bag{"type": standard.LoggerDiscard},
			},
		}))
		testutil.Equal(t, "default", bag.Get(database.DefaultConfigConfigPath), any(standard.Id))
	})

	t.Run("should fail to configure a bag refusing the defaults", func(t *testing.T) {
		bag := container.Bag{"flam": "scalar"}
		if err := standard.NewBundle().(flam.IConfigurableBundle).Config(bag); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(standard.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.database.configs.standard", "provider/1"})
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should keep the gorm defaults of the missing fields", func(t *testing.T) {
		cfg, err := create(t, container.Bag{})
		testutil.NoError(t, err)

		testutil.Equal(t, "Id()", cfg.Id(), "cfg")
		testutil.Equal(t, "SkipDefaultTransaction", cfg.SkipDefaultTransaction, false)
		testutil.Equal(t, "CreateBatchSize", cfg.CreateBatchSize, 0)
		testutil.Equal(t, "PrepareStmtTTL", cfg.PrepareStmtTTL, time.Duration(0))
		lc, out := loggerOf(t, cfg.Logger)
		testutil.Equal(t, "logger", lc, logger.Config{SlowThreshold: 200 * time.Millisecond, LogLevel: logger.Warn})
		testutil.Equal(t, "out", out, io.Writer(os.Stdout))
	})

	t.Run("should set every field", func(t *testing.T) {
		cfg, err := create(t, container.Bag{
			standard.ConfigFieldSkipDefaultTransaction:                   true,
			standard.ConfigFieldFullSaveAssociations:                     true,
			standard.ConfigFieldDryRun:                                   true,
			standard.ConfigFieldPrepareStmt:                              true,
			standard.ConfigFieldPrepareStmtMaxSize:                       uint8(7),
			standard.ConfigFieldPrepareStmtTTL:                           "1m",
			standard.ConfigFieldDisableAutomaticPing:                     true,
			standard.ConfigFieldDisableForeignKeyConstraintWhenMigrating: true,
			standard.ConfigFieldIgnoreRelationshipsWhenMigrating:         true,
			standard.ConfigFieldDisableNestedTransaction:                 true,
			standard.ConfigFieldAllowGlobalUpdate:                        true,
			standard.ConfigFieldQueryFields:                              true,
			standard.ConfigFieldCreateBatchSize:                          100.0,
			standard.ConfigFieldTranslateError:                           true,
			standard.ConfigFieldPropagateUnscoped:                        true,
			standard.ConfigFieldLogger: container.Bag{
				"type": "DISCARD", "level": "Info", "slow_threshold": time.Second, "colorful": true,
				"ignore_record_not_found_error": true, "parameterized_queries": true,
			},
		})
		testutil.NoError(t, err)

		c := cfg.Config
		for name, got := range map[string]bool{
			"SkipDefaultTransaction": c.SkipDefaultTransaction, "FullSaveAssociations": c.FullSaveAssociations,
			"DryRun": c.DryRun, "PrepareStmt": c.PrepareStmt, "DisableAutomaticPing": c.DisableAutomaticPing,
			"DisableForeignKeyConstraintWhenMigrating": c.DisableForeignKeyConstraintWhenMigrating,
			"IgnoreRelationshipsWhenMigrating":         c.IgnoreRelationshipsWhenMigrating,
			"DisableNestedTransaction":                 c.DisableNestedTransaction, "AllowGlobalUpdate": c.AllowGlobalUpdate,
			"QueryFields": c.QueryFields, "TranslateError": c.TranslateError, "PropagateUnscoped": c.PropagateUnscoped,
		} {
			testutil.Equal(t, name, got, true)
		}
		testutil.Equal(t, "PrepareStmtMaxSize", c.PrepareStmtMaxSize, 7)
		testutil.Equal(t, "PrepareStmtTTL", c.PrepareStmtTTL, time.Minute)
		testutil.Equal(t, "CreateBatchSize", c.CreateBatchSize, 100)
		lc, out := loggerOf(t, c.Logger)
		testutil.Equal(t, "logger", lc, logger.Config{
			SlowThreshold: time.Second, Colorful: true, IgnoreRecordNotFoundError: true,
			ParameterizedQueries: true, LogLevel: logger.Info,
		})
		testutil.Equal(t, "out", out, io.Discard)
	})

	t.Run("should read the whole numbers of every kind", func(t *testing.T) {
		for _, val := range []any{int64(9), uint(9), float32(9), 9} {
			cfg, err := create(t, container.Bag{standard.ConfigFieldCreateBatchSize: val})
			testutil.NoError(t, err)
			testutil.Equal(t, "CreateBatchSize", cfg.CreateBatchSize, 9)
		}
	})

	t.Run("should read every logger level", func(t *testing.T) {
		for name, level := range map[string]logger.LogLevel{
			"silent": logger.Silent, "error": logger.Error, "warn": logger.Warn, "info": logger.Info,
		} {
			cfg, err := create(t, container.Bag{standard.ConfigFieldLogger: container.Bag{"level": name, "type": "default"}})
			testutil.NoError(t, err)
			lc, _ := loggerOf(t, cfg.Logger)
			testutil.Equal(t, name, lc.LogLevel, level)
		}
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			standard.ConfigFieldDryRun:              "yes",
			standard.ConfigFieldCreateBatchSize:     1.5,
			standard.ConfigFieldPrepareStmtMaxSize:  uint64(1 << 63),
			standard.ConfigFieldPrepareStmtTTL:      "soon",
			standard.ConfigFieldLoggerSlowThreshold: -time.Second,
			standard.ConfigFieldLoggerLevel:         "loud",
			standard.ConfigFieldLoggerType:          "file",
		} {
			bag := container.Bag{}
			testutil.NoError(t, bag.Set(field, val))
			_, err := create(t, bag)
			var e *flam.Error
			if !errors.Is(err, standard.ErrInvalidField) || !errors.As(errors.Unwrap(err), &e) {
				t.Fatalf("%s: Get() = %v, want ErrInvalidField", field, err)
			}
			testutil.Context(t, e, map[string]any{standard.ErrorFieldID: "cfg", standard.ErrorFieldField: field})
		}
	})

	t.Run("should report the first invalid field", func(t *testing.T) {
		_, err := create(t, container.Bag{
			standard.ConfigFieldDryRun:          1,
			standard.ConfigFieldCreateBatchSize: "x",
			standard.ConfigFieldLogger:          container.Bag{"type": 3},
		})
		testutil.Error(t, errors.Unwrap(err), standard.ErrInvalidField,
			"unable to create standard database config: invalid standard database config field", map[string]any{
				standard.ErrorFieldID: "cfg", standard.ErrorFieldField: standard.ConfigFieldDryRun,
			})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := create(t, container.Bag{factory.ConfigFieldDriver: "other"})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
