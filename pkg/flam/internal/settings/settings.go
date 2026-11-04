// Package settings holds the helpers reading the fields of the resource
// configurations shared by the flam drivers.
package settings

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Field returns the value of the field of the resource configuration, or
// else the one stored in the configuration at the default path, unless
// empty, or else the built-in default, as the field is optional.
func Field(
	config flam.IConfig,
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	switch {
	case bag.Has(field):
		return bag.Get(field)
	case defaultPath != "" && config.Has(defaultPath):
		return config.Get(defaultPath)
	}
	return builtIn
}

// Duration returns the non-negative duration of the value, a
// time.Duration or a string such as "10s", and whether the value is one.
func Duration(
	value any,
) (time.Duration, bool) {
	d := time.Duration(-1)
	switch v := value.(type) {
	case time.Duration:
		d = v
	case string:
		if parsed, err := time.ParseDuration(v); err == nil {
			d = parsed
		}
	}
	return d, d >= 0
}

// Strings returns the strings of the list, a []string or a []any of
// strings, nil being an empty list, and whether the value is one.
func Strings(
	value any,
) ([]string, bool) {
	switch list := value.(type) {
	case nil:
		return nil, true
	case []string:
		return list, true
	case []any:
		items := make([]string, len(list))
		for i, item := range list {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			items[i] = s
		}
		return items, true
	}
	return nil, false
}
