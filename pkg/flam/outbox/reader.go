package outbox

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// reader reads the fields of a relay configuration, taking the defaults
// of the missing ones from the configuration, and recording the first
// invalid one, after which it keeps reading, returning zero values for
// the invalid ones.
type reader struct {
	config  flam.IConfig
	bag     flam.IBag
	invalid string
}

// string returns the string of the field, or of its default, which may be
// empty unless nonEmpty.
func (r *reader) string(
	field string,
	defaultPath string,
	builtIn string,
	nonEmpty bool,
) string {
	s, ok := r.value(field, defaultPath, builtIn).(string)
	r.check(field, ok && (!nonEmpty || s != ""))
	return s
}

// bool returns the bool of the field, or of its default.
func (r *reader) bool(
	field string,
	defaultPath string,
	builtIn bool,
) bool {
	b, ok := r.value(field, defaultPath, builtIn).(bool)
	r.check(field, ok)
	return b
}

// int returns the int of the field, or of its default, which must be min
// or more.
func (r *reader) int(
	field string,
	defaultPath string,
	builtIn int,
	min int,
) int {
	n, ok := r.value(field, defaultPath, builtIn).(int)
	r.check(field, ok && n >= min)
	return n
}

// duration returns the duration of the field, or of its default, a
// time.Duration or a string such as "10s", which must be positive when
// positive is set, and must not be negative otherwise.
func (r *reader) duration(
	field string,
	defaultPath string,
	builtIn time.Duration,
	positive bool,
) time.Duration {
	d, ok := settings.Duration(r.value(field, defaultPath, builtIn))
	r.check(field, ok && (!positive || d > 0))
	return d
}

// check records the field as invalid unless ok, and unless another one
// already is.
func (r *reader) check(
	field string,
	ok bool,
) {
	if !ok && r.invalid == "" {
		r.invalid = field
	}
}

// value returns the value of the field, or else the one stored in the
// configuration at the default path, or else the built-in default.
func (r *reader) value(
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(r.config, r.bag, field, defaultPath, builtIn)
}
