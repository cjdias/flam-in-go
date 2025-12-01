package flam

import "strings"

type LogLevel int

const (
	LogNone LogLevel = iota
	LogFatal
	LogError
	LogWarning
	LogNotice
	LogInfo
	LogDebug
)

func (level LogLevel) String() string {
	switch level {
	case LogFatal:
		return "fatal"
	case LogError:
		return "error"
	case LogWarning:
		return "warning"
	case LogNotice:
		return "notice"
	case LogInfo:
		return "info"
	case LogDebug:
		return "debug"
	default:
		return "none"
	}
}

func LogLevelFrom(
	val any,
	def ...LogLevel,
) LogLevel {
	switch v := val.(type) {
	case LogLevel:
		return v
	case int:
		if v >= int(LogNone) && v <= int(LogDebug) {
			return LogLevel(v)
		}
	case string:
		switch strings.ToLower(v) {
		case "none":
			return LogNone
		case "fatal":
			return LogFatal
		case "error":
			return LogError
		case "warning":
			return LogWarning
		case "notice":
			return LogNotice
		case "info":
			return LogInfo
		case "debug":
			return LogDebug
		}
	}

	return append(def, LogNone)[0]
}
