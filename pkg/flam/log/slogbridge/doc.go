// Package slogbridge bridges the standard [log/slog] package to the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of flam: the
// [log/slog.Handler] returned from [NewHandler] signals each record it
// handles on its channel of the logger, and [NewLogger] returns the
// [log/slog.Logger] of such a handler.
//
// The level of a record is mapped to the flam level by [FlamLevel], and
// the handler only handles the records of its minimum level or above. The
// attributes of a record, and those given to WithAttrs, are stored in the
// context of the entry, each group as a nested bag, following the rules of
// [log/slog.Handler]: an empty attribute and a group without attribute are
// left out, the attributes of a group of empty key are inlined, and the
// values are resolved. When the logger is a
// [github.com/cjdias/flam-in-go/pkg/flam.IContextLogger], as the one of
// the log bundle is, the record is signaled with the context it is logged
// with, so that its entry also holds the fields the extractors of the
// logger extract from it, such as the ids of its OpenTelemetry trace and
// span.
//
// The logger stamps the entry with its own clock: the time of the record
// is stored in the context of the entry only at the time field of the
// handler, when set.
//
// The bundle returned from [NewBundle] provides the [log/slog.Logger] of
// the logger of the container, configured at [ChannelConfigPath],
// [LevelConfigPath] and [TimeFieldConfigPath], and, when
// [SetDefaultConfigPath] holds true, makes it the default logger of
// [log/slog] when the application boots.
//
// The package is named slogbridge, so as not to clash with [log/slog].
package slogbridge
