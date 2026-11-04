package base

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// NewReader returns the reader of the fields of the broker configuration,
// taking the defaults of the missing ones from the configuration.
func NewReader(
	config flam.IConfig,
	bag flam.IBag,
) *Reader {
	return &Reader{config: config, bag: bag}
}

// Reader reads the fields of a broker configuration, recording the first
// invalid one, after which it keeps reading, returning zero values for the
// invalid ones.
type Reader struct {
	config  flam.IConfig
	bag     flam.IBag
	invalid string
}

// Invalid returns the first invalid field read, or "" when none is.
func (r *Reader) Invalid() string {
	return r.invalid
}

// String returns the string of the field, or of its default, which may be
// empty unless nonEmpty.
func (r *Reader) String(
	field string,
	defaultPath string,
	builtIn string,
	nonEmpty bool,
) string {
	s, ok := r.value(field, defaultPath, builtIn).(string)
	if !ok || (nonEmpty && s == "") {
		r.fail(field)
		return ""
	}
	return s
}

// OneOf returns the string of the field, or of its default, which must be
// one of the values.
func (r *Reader) OneOf(
	field string,
	defaultPath string,
	builtIn string,
	values ...string,
) string {
	s, _ := r.value(field, defaultPath, builtIn).(string)
	for _, value := range values {
		if s == value {
			return s
		}
	}
	r.fail(field)
	return ""
}

// Bool returns the bool of the field, or of its default.
func (r *Reader) Bool(
	field string,
	defaultPath string,
	builtIn bool,
) bool {
	b, ok := r.value(field, defaultPath, builtIn).(bool)
	if !ok {
		r.fail(field)
	}
	return b
}

// Int returns the int of the field, or of its default, which must be min
// or more.
func (r *Reader) Int(
	field string,
	defaultPath string,
	builtIn int,
	min int,
) int {
	n, ok := r.value(field, defaultPath, builtIn).(int)
	if !ok || n < min {
		r.fail(field)
		return 0
	}
	return n
}

// Duration returns the non-negative duration of the field, or of its
// default, a time.Duration or a string such as "10s".
func (r *Reader) Duration(
	field string,
	defaultPath string,
	builtIn time.Duration,
) time.Duration {
	d, ok := settings.Duration(r.value(field, defaultPath, builtIn))
	if !ok {
		r.fail(field)
		return 0
	}
	return d
}

// Strings returns the strings of the field, or of its default, a list of
// strings, which may be empty unless nonEmpty.
func (r *Reader) Strings(
	field string,
	defaultPath string,
	builtIn []string,
	nonEmpty bool,
) []string {
	list, ok := settings.Strings(r.value(field, defaultPath, builtIn))
	if !ok || (nonEmpty && len(list) == 0) {
		r.fail(field)
		return nil
	}
	return list
}

// Check records the field as invalid unless ok, for the checks the other
// methods do not make, such as a duration that must be positive.
func (r *Reader) Check(
	field string,
	ok bool,
) {
	if !ok {
		r.fail(field)
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
// configuration at the default path, or else the built-in default.
func (r *Reader) value(
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(r.config, r.bag, field, defaultPath, builtIn)
}
