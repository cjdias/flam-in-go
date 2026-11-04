// Package log is the log feature: the bundle returned from [NewBundle]
// provides the logger of an application, the implementation of the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] contract, and the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogSerializerFactory] and
// [github.com/cjdias/flam-in-go/pkg/flam.ILogStreamFactory], which create
// the serializers and streams described by the configuration at
// [SerializerConfigPath] and [StreamConfigPath] with the creators provided
// in the [SerializerCreatorGroup] and [StreamCreatorGroup] container
// groups.
//
// The logger, also provided as the
// [github.com/cjdias/flam-in-go/pkg/flam.IContextLogger] of the
// container, adds to the entries given with a context the fields the
// extractors of the [ContextExtractorGroup] group extract from it, such
// as the ids of its OpenTelemetry trace and span, extracted by the
// extractor the bundle provides. It keeps its entries until flushed, then
// writes them to the streams stored in the stream factory. When the application boots, the
// bundle creates the streams configured at [StreamConfigPath], unless
// [BootConfigPath] holds false, and flushes the logger at the period
// stored at [FlusherPeriodConfigPath], and a last time when the
// application closes.
//
// The [github.com/cjdias/flam-in-go/pkg/flam/log/slogbridge] package
// bridges the standard [log/slog] package to the logger.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/log/serializers] and
// [github.com/cjdias/flam-in-go/pkg/flam/log/streams]. They import this
// package for its paths and groups, so this package never imports them.
package log
