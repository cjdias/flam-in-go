package slogbridge

import (
	"log/slog"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// FlamLevel returns the flam level of the [log/slog] level: debug below
// [log/slog.LevelInfo], info below [LevelNotice], notice below
// [log/slog.LevelWarn], warning below [log/slog.LevelError], error below
// [LevelFatal], and fatal from it.
func FlamLevel(
	level slog.Level,
) flam.LogLevel {
	switch {
	case level < slog.LevelInfo:
		return flam.LogDebug
	case level < LevelNotice:
		return flam.LogInfo
	case level < slog.LevelWarn:
		return flam.LogNotice
	case level < slog.LevelError:
		return flam.LogWarning
	case level < LevelFatal:
		return flam.LogError
	default:
		return flam.LogFatal
	}
}

// SlogLevel returns the [log/slog] level standing for the flam level,
// which [FlamLevel] maps back to it: [log/slog.LevelDebug],
// [log/slog.LevelInfo], [LevelNotice], [log/slog.LevelWarn],
// [log/slog.LevelError] or [LevelFatal], or [LevelNone] for the none
// level, or a level that is not one.
func SlogLevel(
	level flam.LogLevel,
) slog.Level {
	switch level {
	case flam.LogDebug:
		return slog.LevelDebug
	case flam.LogInfo:
		return slog.LevelInfo
	case flam.LogNotice:
		return LevelNotice
	case flam.LogWarning:
		return slog.LevelWarn
	case flam.LogError:
		return slog.LevelError
	case flam.LogFatal:
		return LevelFatal
	default:
		return LevelNone
	}
}
