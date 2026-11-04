// Package otlp is the log stream driver of the OpenTelemetry collectors:
// its streams export each entry they keep as an OpenTelemetry log record,
// in batches, to the OTLP/HTTP endpoint of a collector, with the protobuf
// encoding.
//
// A record holds the timestamp of its entry, the severity of its level,
// the upper case name of its level as severity text, its message as body,
// and the values of its context as attributes, the bags and maps of
// string keys as maps, the slices and arrays as slices, the numbers,
// booleans, strings and byte slices as such, and the other values as
// strings: a [time.Time] formatted as [time.RFC3339Nano], an error as its
// message, a [fmt.Stringer] as its string, and any other value as
// formatted by fmt %v. The levels map to the severities as follows: fatal
// to FATAL, error to ERROR, warning to WARN, notice to INFO2, info to INFO
// and debug to DEBUG. The records are given the resource of the service
// name of the stream, when set.
//
// A stream keeps the entries of its level or more severe, an entry of
// level [github.com/cjdias/flam-in-go/pkg/flam.LogNone] never being kept:
// the broadcast ones, and the signaled ones when it listens to their
// channel, or to "*". The channel of a signaled entry is given in the
// context, at [github.com/cjdias/flam-in-go/pkg/flam/log.ContextFieldChannel],
// and so as an attribute.
//
// The records are exported in the background, so that a failing collector
// does not fail the writes: the exporter retries a failed export, and
// reports the export failing nonetheless to the error handler of the
// OpenTelemetry global package. Closing a stream exports the records not
// exported yet, within its timeout, then shuts its exporter down.
package otlp
