package flam

import (
	"io"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

type defaultDatabaseConfigCreator struct{}

func newDefaultDatabaseConfigCreator() DatabaseConfigCreator {
	return &defaultDatabaseConfigCreator{}
}

var _ DatabaseConfigCreator = (*defaultDatabaseConfigCreator)(nil)

func (defaultDatabaseConfigCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == DatabaseConfigDriverDefault
}

func (creator defaultDatabaseConfigCreator) Create(
	config Bag,
) (DatabaseConfig, error) {
	configLogger, e := creator.getLogger(config)
	if e != nil {
		return nil, e
	}

	var prepareStmtTTL time.Duration
	if PrepareStmtTTL := config.String("prepare_stmt_ttl"); PrepareStmtTTL != "" {
		prepareStmtTTL, e = time.ParseDuration(PrepareStmtTTL)
		if e != nil {
			return nil, e
		}
	}

	return &gorm.Config{
		SkipDefaultTransaction:                   config.Bool("skip_default_transaction"),
		FullSaveAssociations:                     config.Bool("full_save_associations"),
		Logger:                                   configLogger,
		DryRun:                                   config.Bool("dry_run"),
		PrepareStmt:                              config.Bool("prepare_stmt"),
		PrepareStmtMaxSize:                       config.Int("prepare_stmt_max_size"),
		PrepareStmtTTL:                           prepareStmtTTL,
		DisableAutomaticPing:                     config.Bool("disable_automatic_ping"),
		DisableForeignKeyConstraintWhenMigrating: config.Bool("disable_foreign_key_constraint_when_migrating"),
		IgnoreRelationshipsWhenMigrating:         config.Bool("ignore_relationships_when_migrating"),
		DisableNestedTransaction:                 config.Bool("disable_nested_transaction"),
		AllowGlobalUpdate:                        config.Bool("allow_global_update"),
		QueryFields:                              config.Bool("query_fields"),
		CreateBatchSize:                          config.Int("create_batch_size"),
		TranslateError:                           config.Bool("translate_error"),
		PropagateUnscoped:                        config.Bool("propagate_unscoped"),
	}, nil
}

func (creator defaultDatabaseConfigCreator) getLogger(
	config Bag,
) (gormLogger.Interface, error) {
	cfg := gormLogger.Config{
		Colorful:                  config.Bool("logger.colorful"),
		IgnoreRecordNotFoundError: config.Bool("logger.ignore_record_not_found_error"),
		ParameterizedQueries:      config.Bool("logger.parameterized_queries"),
	}

	if SlowThreshold := config.String("logger.slow_threshold"); SlowThreshold != "" {
		slowThreshold, e := time.ParseDuration(SlowThreshold)
		if e != nil {
			return nil, e
		}
		cfg.SlowThreshold = slowThreshold
	}

	if level := config.String("logger.level"); level != "" {
		switch strings.ToLower(level) {
		case "silent":
			cfg.LogLevel = gormLogger.Silent
		case "error":
			cfg.LogLevel = gormLogger.Error
		case "", "warn":
			cfg.LogLevel = gormLogger.Warn
		case "info":
			cfg.LogLevel = gormLogger.Info
		default:
			return nil, newErrUnknownDatabaseLogLevel(level)
		}
	}

	loggerType := strings.ToLower(config.String("logger.type", DatabaseConfigLoggerDefault))
	switch loggerType {
	case DatabaseConfigLoggerDefault:
		return gormLogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), cfg), nil
	case DatabaseConfigLoggerDiscard:
		return gormLogger.New(log.New(io.Discard, "", log.LstdFlags), cfg), nil
	}

	return nil, newErrUnknownDatabaseLogType(loggerType)
}
