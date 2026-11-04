// Package none is the tracing exporter driver discarding the spans, such
// as for an environment whose configuration lists an exporter at
// [github.com/cjdias/flam-in-go/pkg/flam/tracing.ExportConfigPath] while
// no collector receives the spans: the spans are still sampled and
// recorded, for the instrumentations reading them, such as the trace id
// written to the logs, but exported nowhere. To record no span at all,
// disable the tracing at
// [github.com/cjdias/flam-in-go/pkg/flam/tracing.EnabledConfigPath]
// instead.
package none
