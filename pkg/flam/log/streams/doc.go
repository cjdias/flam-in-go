// Package streams provides the bundle registering every log stream driver
// bundle, to be registered along with the
// [github.com/cjdias/flam-in-go/pkg/flam/log] bundle, while its sub
// packages hold the stream drivers: console, file, rotating_file, syslog,
// registered only on the platforms providing the log/syslog package, that
// is all but windows and plan9, and otlp.
package streams
