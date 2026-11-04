package standard

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.database.configs.standard"

const (
	// Id is the id of the default gorm configuration configured by the
	// bundle returned from [NewBundle], and the default of the
	// configuration of the connections.
	Id = "standard"

	// Driver is the driver name of the standard configurations, held by
	// the [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.database.configs.driver.standard"
)

// Fields of a standard configuration, each setting the gorm.Config field
// of the same name: a bool, a whole number, or a time.Duration or a string
// such as "1m" for PrepareStmtTTL. A missing field keeps the gorm default.
const (
	ConfigFieldSkipDefaultTransaction                   = "skip_default_transaction"
	ConfigFieldFullSaveAssociations                     = "full_save_associations"
	ConfigFieldDryRun                                   = "dry_run"
	ConfigFieldPrepareStmt                              = "prepare_stmt"
	ConfigFieldPrepareStmtMaxSize                       = "prepare_stmt_max_size"
	ConfigFieldPrepareStmtTTL                           = "prepare_stmt_ttl"
	ConfigFieldDisableAutomaticPing                     = "disable_automatic_ping"
	ConfigFieldDisableForeignKeyConstraintWhenMigrating = "disable_foreign_key_constraint_when_migrating"
	ConfigFieldIgnoreRelationshipsWhenMigrating         = "ignore_relationships_when_migrating"
	ConfigFieldDisableNestedTransaction                 = "disable_nested_transaction"
	ConfigFieldAllowGlobalUpdate                        = "allow_global_update"
	ConfigFieldQueryFields                              = "query_fields"
	ConfigFieldCreateBatchSize                          = "create_batch_size"
	ConfigFieldTranslateError                           = "translate_error"
	ConfigFieldPropagateUnscoped                        = "propagate_unscoped"
)

// Fields of the "logger" field of a standard configuration.
const (
	// ConfigFieldLogger is the field holding the configuration of the gorm
	// logger.
	ConfigFieldLogger = "logger"

	// ConfigFieldLoggerType is the field holding the type of the logger,
	// [LoggerDefault] or [LoggerDiscard], LoggerDefault when missing.
	ConfigFieldLoggerType = "logger.type"

	// ConfigFieldLoggerLevel is the field holding the level of the logger,
	// "silent", "error", "warn" or "info", whatever its case, "warn" when
	// missing.
	ConfigFieldLoggerLevel = "logger.level"

	// ConfigFieldLoggerSlowThreshold is the field holding the duration from
	// which a query is logged as slow, a time.Duration or a string such as
	// "200ms", 200ms when missing.
	ConfigFieldLoggerSlowThreshold = "logger.slow_threshold"

	// ConfigFieldLoggerColorful is the field holding whether the logger
	// colours its lines.
	ConfigFieldLoggerColorful = "logger.colorful"

	// ConfigFieldLoggerIgnoreRecordNotFoundError is the field holding
	// whether the logger ignores gorm.ErrRecordNotFound.
	ConfigFieldLoggerIgnoreRecordNotFoundError = "logger.ignore_record_not_found_error"

	// ConfigFieldLoggerParameterizedQueries is the field holding whether
	// the logger leaves the parameters out of the logged queries.
	ConfigFieldLoggerParameterizedQueries = "logger.parameterized_queries"
)

// Types of the gorm logger of a standard configuration.
const (
	// LoggerDefault writes the gorm logs to the standard output.
	LoggerDefault = "default"

	// LoggerDiscard discards the gorm logs.
	LoggerDiscard = "discard"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the configuration.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the logger fields.
const (
	defaultLoggerType          = LoggerDefault
	defaultLoggerLevel         = "warn"
	defaultLoggerSlowThreshold = 200 * time.Millisecond
)
