package log

import (
	"time"
)

// bundleId is the id of the log bundle.
const bundleId = "flam.log"

// Configuration paths and container groups of the serializers and streams.
const (
	// SerializerConfigPath is the configuration path holding one
	// serializer configuration per serializer id, as read by the
	// serializer factory.
	SerializerConfigPath = "flam.log.serializers"

	// SerializerCreatorGroup is the container group the serializer bundles
	// provide their [flam.ILogSerializerCreator] in, collected by the
	// serializer factory.
	SerializerCreatorGroup = "flam.log.serializers.creator"

	// StreamConfigPath is the configuration path holding one stream
	// configuration per stream id, as read by the stream factory.
	StreamConfigPath = "flam.log.streams"

	// StreamCreatorGroup is the container group the stream bundles provide
	// their [flam.ILogStreamCreator] in, collected by the stream factory.
	StreamCreatorGroup = "flam.log.streams.creator"

	// ContextExtractorGroup is the container group the
	// [flam.ILogContextExtractor] values are provided in, collected by the
	// logger, which merges the fields they extract from the context an
	// entry is given in into the context of the entry. The log bundle
	// provides in it the extractor of the trace and span ids.
	ContextExtractorGroup = "flam.log.context.extractors"
)

// Configuration paths of the fields of the trace extractor.
const (
	// TraceIdFieldConfigPath is the configuration path of the context
	// field the trace extractor stores the id of the OpenTelemetry trace
	// of the context at, as a hexadecimal string. It defaults to
	// [ContextFieldTraceId], and an empty field leaves the id out. It is
	// read when the logger is created.
	TraceIdFieldConfigPath = "flam.log.context.trace_id"

	// SpanIdFieldConfigPath is the configuration path of the context
	// field the trace extractor stores the id of the OpenTelemetry span of
	// the context at, as a hexadecimal string. It defaults to
	// [ContextFieldSpanId], and an empty field leaves the id out. It is
	// read when the logger is created.
	SpanIdFieldConfigPath = "flam.log.context.span_id"
)

// Configuration paths of the log bundle behaviour.
const (
	// BootConfigPath is the configuration path of a bool telling whether
	// the log bundle, when the application boots, creates every stream
	// configured at [StreamConfigPath] with the stream factory, so that the
	// logger writes to it. It defaults to true.
	BootConfigPath = "flam.log.boot"

	// FlusherPeriodConfigPath is the configuration path of the period, a
	// time.Duration or a string such as "1s", at which the log bundle
	// flushes the [flam.ILogger], with a recurring [flam.ITrigger] of id
	// [FlusherTriggerID]. It defaults to one second, also taken for a value
	// that is not a period. A period of zero or less disables the periodic
	// flush: the entries are then kept in memory until Flush is called, or
	// the application closes. The bundle observes the path, with an observer of id
	// [FlusherTriggerID], and restarts the trigger when the period changes.
	FlusherPeriodConfigPath = "flam.log.flusher.period"

	// FlusherTriggerID is the id of the trigger flushing the logger, and of
	// the configuration observer of [FlusherPeriodConfigPath].
	FlusherTriggerID = "flam.log.flusher"
)

// Context fields of the [*flam.Error] values returned by the log bundle,
// and of the entries it logs.
const (
	// ErrorFieldID holds the id of a stream.
	ErrorFieldID = "id"

	// ErrorFieldPeriod holds the flush period, a time.Duration.
	ErrorFieldPeriod = "period"

	// ErrorFieldLevel holds a log level.
	ErrorFieldLevel = "level"

	// ErrorFieldChannel holds a log channel.
	ErrorFieldChannel = "channel"

	// ErrorFieldError holds, in the context of the entries the flusher
	// broadcasts when a periodic flush, or a restart of its trigger,
	// fails, the error of that flush or restart.
	ErrorFieldError = "error"
)

// Channels and context fields of the streams of the drivers.
const (
	// AllChannels is the channel a stream listens to in order to keep the
	// entries signaled on any channel.
	AllChannels = "*"

	// ContextFieldChannel is the field of the context given to the
	// serializer holding the channel of a signaled entry, over a context
	// value of the same key.
	ContextFieldChannel = "channel"

	// ContextFieldTraceId is the default field of the id of the trace of
	// the context of an entry, set by [TraceIdFieldConfigPath].
	ContextFieldTraceId = "trace_id"

	// ContextFieldSpanId is the default field of the id of the span of the
	// context of an entry, set by [SpanIdFieldConfigPath].
	ContextFieldSpanId = "span_id"
)

// Defaults of the log bundle behaviour.
const (
	defaultBoot        = true
	defaultFlushPeriod = time.Second
)
