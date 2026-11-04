package otlp

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// writer is the base.Writer of a stream, emitting the record of the entry
// its recorder records just before: the stream serializes an entry and
// writes its line holding its lock, so that the entry recorded is the one
// of the line written, which is ignored.
type writer struct {
	provider *sdklog.LoggerProvider
	logger   otellog.Logger
	timeout  time.Duration
	record   otellog.Record // of the entry being written
}

// WriteLine emits the record of the entry, to be exported in the
// background.
func (w *writer) WriteLine(
	time.Time,
	string,
) error {
	w.logger.Emit(context.Background(), w.record)
	w.record = otellog.Record{}
	return nil
}

// Close exports the records not exported yet, within the timeout, then
// shuts the exporter down, returning the error of the export.
func (w *writer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()
	return w.provider.Shutdown(ctx)
}

// recorder is the serializer of a stream, recording the record of each
// entry in the writer, and returning no line. The stream never closes its
// serializer nor asks for its id: it takes those methods from the nil
// serializer it embeds, never called.
type recorder struct {
	flam.ILogSerializer
	w *writer
}

func (r recorder) Serialize(
	timestamp time.Time,
	level flam.LogLevel,
	message string,
	ctx flam.IBag,
) string {
	var record otellog.Record
	record.SetTimestamp(timestamp)
	record.SetSeverity(severities[level])
	record.SetSeverityText(strings.ToUpper(level.String()))
	record.SetBody(attribute.StringValue(message))
	for key, val := range ctx.All() {
		record.AddAttributes(attribute.KeyValue{Key: attribute.Key(key), Value: valueOf(val)})
	}
	r.w.record = record
	return ""
}

// severities are the severities of the levels the streams keep.
var severities = map[flam.LogLevel]otellog.Severity{
	flam.LogFatal:   otellog.SeverityFatal,
	flam.LogError:   otellog.SeverityError,
	flam.LogWarning: otellog.SeverityWarn,
	flam.LogNotice:  otellog.SeverityInfo2,
	flam.LogInfo:    otellog.SeverityInfo,
	flam.LogDebug:   otellog.SeverityDebug,
}

// valueOf returns the attribute value of a context value, as documented
// by the package.
func valueOf(
	val any,
) attribute.Value {
	switch v := val.(type) {
	case nil:
		return attribute.Value{}
	case string:
		return attribute.StringValue(v)
	case bool:
		return attribute.BoolValue(v)
	case []byte:
		return attribute.ByteSliceValue(v)
	case time.Time:
		return attribute.StringValue(v.Format(time.RFC3339Nano))
	case error:
		return attribute.StringValue(v.Error())
	case fmt.Stringer:
		return attribute.StringValue(v.String())
	case flam.IBag:
		var kvs []attribute.KeyValue
		for key, item := range v.All() {
			kvs = append(kvs, attribute.KeyValue{Key: attribute.Key(key), Value: valueOf(item)})
		}
		return attribute.MapValue(kvs...)
	}

	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.String:
		return attribute.StringValue(rv.String())
	case reflect.Bool:
		return attribute.BoolValue(rv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return attribute.Int64Value(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if n := rv.Uint(); n <= math.MaxInt64 {
			return attribute.Int64Value(int64(n))
		}
	case reflect.Float32, reflect.Float64:
		return attribute.Float64Value(rv.Float())
	case reflect.Slice, reflect.Array:
		values := make([]attribute.Value, rv.Len())
		for i := range values {
			values[i] = valueOf(rv.Index(i).Interface())
		}
		return attribute.SliceValue(values...)
	case reflect.Map:
		if rv.Type().Key().Kind() == reflect.String {
			keys := make([]string, 0, rv.Len())
			items := map[string]any{}
			for iter := rv.MapRange(); iter.Next(); {
				key := iter.Key().String()
				keys = append(keys, key)
				items[key] = iter.Value().Interface()
			}
			slices.Sort(keys)
			kvs := make([]attribute.KeyValue, len(keys))
			for i, key := range keys {
				kvs[i] = attribute.KeyValue{Key: attribute.Key(key), Value: valueOf(items[key])}
			}
			return attribute.MapValue(kvs...)
		}
	}
	return attribute.StringValue(fmt.Sprint(val))
}
