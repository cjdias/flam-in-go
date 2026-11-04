package standard

import (
	"io"
	"log"
	"math"
	"os"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IDatabaseConfigCreator] of the standard
// configurations. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator() flam.IDatabaseConfigCreator {
	return &creator{}
}

// creator is the [flam.IDatabaseConfigCreator] of the configurations with
// the [Driver] driver.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the gorm configuration of the fields of the
// configuration, failing at the first invalid one.
func (c *creator) Create(
	bag flam.IBag,
) (flam.DatabaseConfig, error) {
	id := bag.String(factory.ConfigFieldId)
	r := &reader{bag: bag}

	config := gorm.Config{
		SkipDefaultTransaction:                   r.bool(ConfigFieldSkipDefaultTransaction),
		FullSaveAssociations:                     r.bool(ConfigFieldFullSaveAssociations),
		DryRun:                                   r.bool(ConfigFieldDryRun),
		PrepareStmt:                              r.bool(ConfigFieldPrepareStmt),
		PrepareStmtMaxSize:                       r.int(ConfigFieldPrepareStmtMaxSize),
		PrepareStmtTTL:                           r.duration(ConfigFieldPrepareStmtTTL, 0),
		DisableAutomaticPing:                     r.bool(ConfigFieldDisableAutomaticPing),
		DisableForeignKeyConstraintWhenMigrating: r.bool(ConfigFieldDisableForeignKeyConstraintWhenMigrating),
		IgnoreRelationshipsWhenMigrating:         r.bool(ConfigFieldIgnoreRelationshipsWhenMigrating),
		DisableNestedTransaction:                 r.bool(ConfigFieldDisableNestedTransaction),
		AllowGlobalUpdate:                        r.bool(ConfigFieldAllowGlobalUpdate),
		QueryFields:                              r.bool(ConfigFieldQueryFields),
		CreateBatchSize:                          r.int(ConfigFieldCreateBatchSize),
		TranslateError:                           r.bool(ConfigFieldTranslateError),
		PropagateUnscoped:                        r.bool(ConfigFieldPropagateUnscoped),
		Logger:                                   r.logger(),
	}
	if r.invalid != "" {
		return flam.DatabaseConfig{}, newErrInvalidField(creatorCreateMessage, id, r.invalid)
	}
	return flam.NewDatabaseConfig(id, config), nil
}

// reader reads the fields of a configuration, recording the first invalid
// one, after which it reads zero values.
type reader struct {
	bag     flam.IBag
	invalid string
}

// fail records the field as invalid, unless one already is.
func (r *reader) fail(
	field string,
) {
	if r.invalid == "" {
		r.invalid = field
	}
}

// bool returns the bool of the field, false when missing.
func (r *reader) bool(
	field string,
) bool {
	val := r.bag.Get(field)
	b, ok := val.(bool)
	if val != nil && !ok {
		r.fail(field)
	}
	return b
}

// int returns the whole number of the field, of any integer or float kind,
// 0 when missing.
func (r *reader) int(
	field string,
) int {
	val := r.bag.Get(field)
	if val == nil {
		return 0
	}
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if rv.Uint() <= math.MaxInt {
			return int(rv.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if f := rv.Float(); f == math.Trunc(f) && f >= math.MinInt && f < math.MaxInt {
			return int(f)
		}
	}
	r.fail(field)
	return 0
}

// duration returns the non-negative duration of the field, a
// time.Duration or a string such as "1s", def when missing.
func (r *reader) duration(
	field string,
	def time.Duration,
) time.Duration {
	val := r.bag.Get(field)
	if val == nil {
		return def
	}
	duration, ok := val.(time.Duration)
	if text, isString := val.(string); isString {
		var err error
		duration, err = time.ParseDuration(text)
		ok = err == nil
	}
	if !ok || duration < 0 {
		r.fail(field)
		return def
	}
	return duration
}

// text returns the lower case string of the field, def when missing.
func (r *reader) text(
	field string,
	def string,
) string {
	val := r.bag.Get(field)
	if val == nil {
		return def
	}
	s, ok := val.(string)
	if !ok {
		r.fail(field)
		return def
	}
	return strings.ToLower(s)
}

// logger returns the gorm logger of the logger fields.
func (r *reader) logger() logger.Interface {
	config := logger.Config{
		SlowThreshold:             r.duration(ConfigFieldLoggerSlowThreshold, defaultLoggerSlowThreshold),
		Colorful:                  r.bool(ConfigFieldLoggerColorful),
		IgnoreRecordNotFoundError: r.bool(ConfigFieldLoggerIgnoreRecordNotFoundError),
		ParameterizedQueries:      r.bool(ConfigFieldLoggerParameterizedQueries),
	}

	switch r.text(ConfigFieldLoggerLevel, defaultLoggerLevel) {
	case "silent":
		config.LogLevel = logger.Silent
	case "error":
		config.LogLevel = logger.Error
	case "warn":
		config.LogLevel = logger.Warn
	case "info":
		config.LogLevel = logger.Info
	default:
		r.fail(ConfigFieldLoggerLevel)
	}

	var out io.Writer = os.Stdout
	switch r.text(ConfigFieldLoggerType, defaultLoggerType) {
	case LoggerDefault:
	case LoggerDiscard:
		out = io.Discard
	default:
		r.fail(ConfigFieldLoggerType)
	}
	return logger.New(log.New(out, "\r\n", log.LstdFlags), config)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard database config"
