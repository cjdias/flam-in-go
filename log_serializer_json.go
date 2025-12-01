package flam

import (
	"encoding/json"
	"strings"
	"time"
)

type jsonLogSerializer struct{}

var _ LogSerializer = (*jsonLogSerializer)(nil)

func newJsonLogSerializer() LogSerializer {
	return &jsonLogSerializer{}
}

func (jsonLogSerializer) Close() error {
	return nil
}

func (jsonLogSerializer) Serialize(
	timestamp time.Time,
	level LogLevel,
	message string,
	ctx Bag,
) string {
	ctx["time"] = timestamp.Format("2006-01-02T15:04:05.000-0700")
	ctx["level"] = strings.ToUpper(level.String())
	ctx["message"] = message
	bytes, _ := json.Marshal(ctx)
	bytes = append(bytes, '\n')

	return string(bytes)
}
