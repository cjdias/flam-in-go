package flam

import "time"

type LogSerializer interface {
	Close() error

	Serialize(timestamp time.Time, level LogLevel, message string, ctx Bag) string
}
