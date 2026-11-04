package base

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
)

// NewReader returns the reader of the fields of the connection
// configuration, taking the defaults of the missing ones from the
// configuration.
func NewReader(
	config flam.IConfig,
	bag flam.IBag,
) *Reader {
	return &Reader{config: config, bag: bag}
}

// Reader reads the fields of a connection configuration, recording the
// first invalid one, after which it keeps reading, returning zero values
// for the invalid ones.
type Reader struct {
	config  flam.IConfig
	bag     flam.IBag
	invalid string
}

// Invalid returns the first invalid field read, or "" when none is.
func (r *Reader) Invalid() string {
	return r.invalid
}

// NonEmpty returns the non-empty string of the field, or of its default.
func (r *Reader) NonEmpty(
	field string,
	defaultPath string,
	builtIn string,
) string {
	s, ok := r.value(field, defaultPath, builtIn).(string)
	if !ok || s == "" {
		r.fail(field)
		return ""
	}
	return s
}

// Text returns the string of the field, or of its default, which may be
// empty.
func (r *Reader) Text(
	field string,
	defaultPath string,
	builtIn string,
) string {
	s, ok := r.value(field, defaultPath, builtIn).(string)
	if !ok {
		r.fail(field)
	}
	return s
}

// Port returns the port of the field, or of its default, a whole number
// between 1 and 65535 or a string holding one.
func (r *Reader) Port(
	field string,
	defaultPath string,
	builtIn int,
) int {
	val := r.value(field, defaultPath, builtIn)
	if s, ok := val.(string); ok {
		var err error
		if val, err = strconv.Atoi(strings.TrimSpace(s)); err != nil {
			r.fail(field)
			return 0
		}
	}
	n, ok := readerInt(val)
	if !ok || n < 1 || n > math.MaxUint16 {
		r.fail(field)
		return 0
	}
	return n
}

// Options returns the go-redis options of the fields shared by the
// drivers, the username, password and database taking their defaults from
// the configuration at the default paths, when not empty. The fields
// go-redis cannot take a negative value of, such as the pool size, which
// would make it panic, are invalid when negative.
func (r *Reader) Options(
	usernamePath string,
	passwordPath string,
	databasePath string,
) *goredis.Options {
	return &goredis.Options{
		Username:              r.Text(redis.ConfigFieldUsername, usernamePath, ""),
		Password:              r.Text(redis.ConfigFieldPassword, passwordPath, ""),
		DB:                    r.atLeast(redis.ConfigFieldDatabase, databasePath, 0),
		Protocol:              r.protocol(),
		ClientName:            r.Text(redis.ConfigFieldClientName, "", ""),
		MaxRetries:            r.atLeast(redis.ConfigFieldMaxRetries, "", -1),
		MinRetryBackoff:       r.duration(redis.ConfigFieldMinRetryBackoff, true),
		MaxRetryBackoff:       r.duration(redis.ConfigFieldMaxRetryBackoff, true),
		DialTimeout:           r.duration(redis.ConfigFieldDialTimeout, false),
		ReadTimeout:           r.duration(redis.ConfigFieldReadTimeout, true),
		WriteTimeout:          r.duration(redis.ConfigFieldWriteTimeout, true),
		ContextTimeoutEnabled: r.bool(redis.ConfigFieldContextTimeoutEnabled),
		ReadBufferSize:        r.atLeast(redis.ConfigFieldReadBufferSize, "", 0),
		WriteBufferSize:       r.atLeast(redis.ConfigFieldWriteBufferSize, "", 0),
		PoolFIFO:              r.bool(redis.ConfigFieldPoolFIFO),
		PoolSize:              r.atLeast(redis.ConfigFieldPoolSize, "", 0),
		PoolTimeout:           r.duration(redis.ConfigFieldPoolTimeout, false),
		MinIdleConns:          r.atLeast(redis.ConfigFieldMinIdleConns, "", 0),
		MaxIdleConns:          r.atLeast(redis.ConfigFieldMaxIdleConns, "", 0),
		MaxActiveConns:        r.atLeast(redis.ConfigFieldMaxActiveConns, "", 0),
		ConnMaxIdleTime:       r.duration(redis.ConfigFieldConnMaxIdleTime, true),
		ConnMaxLifetime:       r.duration(redis.ConfigFieldConnMaxLifetime, true),
		DisableIdentity:       r.bool(redis.ConfigFieldDisableIdentity),
		IdentitySuffix:        r.Text(redis.ConfigFieldIdentitySuffix, "", ""),
	}
}

// fail records the field as invalid, unless one already is.
func (r *Reader) fail(
	field string,
) {
	if r.invalid == "" {
		r.invalid = field
	}
}

// value returns the value of the field, or else the one stored in the
// configuration at the default path, when not empty, or else the built-in
// default, as the field is optional. A nil value is a missing one.
func (r *Reader) value(
	field string,
	defaultPath string,
	builtIn any,
) any {
	if val := r.bag.Get(field); val != nil {
		return val
	}
	if defaultPath != "" {
		if val := r.config.Get(defaultPath); val != nil {
			return val
		}
	}
	return builtIn
}

// bool returns the bool of the field, false when missing.
func (r *Reader) bool(
	field string,
) bool {
	val := r.value(field, "", false)
	b, ok := val.(bool)
	if !ok {
		r.fail(field)
	}
	return b
}

// atLeast returns the whole number of the field, or of its default, which
// must be min or more.
func (r *Reader) atLeast(
	field string,
	defaultPath string,
	min int,
) int {
	n, ok := readerInt(r.value(field, defaultPath, 0))
	if !ok || n < min {
		r.fail(field)
		return 0
	}
	return n
}

// protocol returns the RESP protocol version of the field, 2 or 3, 0 when
// missing.
func (r *Reader) protocol() int {
	n, ok := readerInt(r.value(redis.ConfigFieldProtocol, "", 0))
	if !ok || (n != 0 && n != 2 && n != 3) {
		r.fail(redis.ConfigFieldProtocol)
		return 0
	}
	return n
}

// duration returns the duration of the field, a time.Duration or a string
// such as "1s", 0 when missing. A negative one is -1, the value go-redis
// reads as disabled, when the field can be disabled, and invalid
// otherwise.
func (r *Reader) duration(
	field string,
	disables bool,
) time.Duration {
	var d time.Duration
	switch val := r.value(field, "", time.Duration(0)).(type) {
	case time.Duration:
		d = val
	case string:
		var err error
		if d, err = time.ParseDuration(strings.TrimSpace(val)); err != nil {
			r.fail(field)
			return 0
		}
	default:
		r.fail(field)
		return 0
	}
	if d < 0 && !disables {
		r.fail(field)
		return 0
	}
	return max(d, -1)
}

// readerInt returns the whole number held by the value, of any integer or
// float kind, or false for any other value or one out of the int range.
func readerInt(
	val any,
) (int, bool) {
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if rv.Uint() <= math.MaxInt {
			return int(rv.Uint()), true
		}
	case reflect.Float32, reflect.Float64:
		if f := rv.Float(); f == math.Trunc(f) && f >= math.MinInt && f < math.MaxInt {
			return int(f), true
		}
	}
	return 0, false
}
