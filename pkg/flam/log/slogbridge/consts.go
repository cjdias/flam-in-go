package slogbridge

import (
	"log/slog"
	"math"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.slog"

// Configuration paths of the logger provided by the bundle.
const (
	// ChannelConfigPath is the configuration path of the channel the
	// records are signaled on, [DefaultChannel] by default.
	ChannelConfigPath = "flam.log.slog.channel"

	// LevelConfigPath is the configuration path of the minimum level of
	// the records handled, a flam level as
	// [github.com/cjdias/flam-in-go/pkg/flam.LogLevel] From reads it, such
	// as "info", the default, also taken for a value that is not a level.
	// The level "none" disables the logger.
	LevelConfigPath = "flam.log.slog.level"

	// TimeFieldConfigPath is the configuration path of the context field
	// the time of the records is stored at, none, leaving it out, by
	// default.
	TimeFieldConfigPath = "flam.log.slog.time_field"

	// SetDefaultConfigPath is the configuration path of a bool telling
	// whether the bundle makes its logger the default logger of
	// [log/slog] when the application boots. It defaults to false.
	SetDefaultConfigPath = "flam.log.slog.set_default"
)

// DefaultChannel is the channel the records are signaled on when the
// handler is given none.
const DefaultChannel = "slog"

// Levels of [log/slog] standing for the flam levels it has no level for.
const (
	// LevelNotice is the level between [log/slog.LevelInfo] and
	// [log/slog.LevelWarn] standing for the notice level.
	LevelNotice = slog.Level(2)

	// LevelFatal is the level above [log/slog.LevelError] standing for
	// the fatal level.
	LevelFatal = slog.Level(12)

	// LevelNone is the level above any other, standing for the none
	// level: as a minimum level, it disables a handler.
	LevelNone = slog.Level(math.MaxInt)
)

// Defaults of the logger provided by the bundle.
const (
	defaultSetDefault = false
)
