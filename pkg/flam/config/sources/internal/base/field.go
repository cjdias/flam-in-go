package base

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// Field returns the value of the field of the source configuration, or
// else the one stored in the configuration at the default path, or else
// the built-in default, as the field is optional. It delegates to the
// shared settings.Field.
func Field(
	config flam.IConfig,
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(config, bag, field, defaultPath, builtIn)
}
