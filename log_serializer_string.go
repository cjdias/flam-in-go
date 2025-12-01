package flam

import (
	"fmt"
	"strings"
	"time"
)

type stringLogSerializer struct{}

var _ LogSerializer = (*stringLogSerializer)(nil)

func newStringLogSerializer() LogSerializer {
	return &stringLogSerializer{}
}

func (stringLogSerializer) Close() error {
	return nil
}

func (stringLogSerializer) Serialize(
	timestamp time.Time,
	level LogLevel,
	message string,
	_ Bag,
) string {
	return fmt.Sprintf(
		"%s [%s] %s\n",
		timestamp.Format("2006-01-02T15:04:05.000-0700"),
		strings.ToUpper(level.String()),
		message)
}
