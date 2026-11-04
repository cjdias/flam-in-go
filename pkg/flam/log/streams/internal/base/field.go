package base

import (
	"slices"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// Field returns the value of the field of the stream configuration, or
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

// Level returns the level held by the value, a flam.LogLevel, an int or a
// level name, as flam.LogLevel From reads it, or false for any other
// value.
func Level(
	val any,
) (flam.LogLevel, bool) {
	const invalid = flam.LogLevel(-1)
	level := flam.LogNone.From(val, invalid)
	return level, level != invalid
}

// Channels returns the channels held by the value, a []string or a []any
// of strings, none empty, or nil, which holds none, or false for any other
// value.
func Channels(
	val any,
) ([]string, bool) {
	switch v := val.(type) {
	case nil:
		return nil, true
	case []string:
		return v, !slices.Contains(v, "")
	case []any:
		channels := make([]string, 0, len(v))
		for _, item := range v {
			channel, ok := item.(string)
			if !ok {
				return nil, false
			}
			channels = append(channels, channel)
		}
		return channels, !slices.Contains(channels, "")
	}
	return nil, false
}

// NonEmpty returns the string held by the value, or false when it is not
// a non-empty string.
func NonEmpty(
	val any,
) (string, bool) {
	s, ok := val.(string)
	return s, ok && s != ""
}

// Fields of the configuration shared by the streams of every driver.
const (
	ConfigFieldLevel      = "level"
	ConfigFieldChannels   = "channels"
	ConfigFieldSerializer = "serializer"
)

// Defaults are the default paths and built-in defaults of the level and
// serializer of the streams of a driver.
type Defaults struct {
	LevelPath      string
	Level          flam.LogLevel
	SerializerPath string
	Serializer     string
}

// Settings are the level, channels and serializer id of a stream.
type Settings struct {
	Level      flam.LogLevel
	Channels   []string
	Serializer string
}

// Read reads the settings of the stream configuration, the missing level
// and serializer taking their defaults, or returns the name of the first
// invalid field.
func Read(
	config flam.IConfig,
	bag flam.IBag,
	defaults Defaults,
) (Settings, string) {
	var settings Settings
	var ok bool
	if settings.Level, ok = Level(Field(config, bag, ConfigFieldLevel, defaults.LevelPath, defaults.Level)); !ok {
		return Settings{}, ConfigFieldLevel
	}
	if settings.Channels, ok = Channels(bag.Get(ConfigFieldChannels)); !ok {
		return Settings{}, ConfigFieldChannels
	}
	if settings.Serializer, ok = NonEmpty(Field(config, bag, ConfigFieldSerializer, defaults.SerializerPath, defaults.Serializer)); !ok {
		return Settings{}, ConfigFieldSerializer
	}
	return settings, ""
}
