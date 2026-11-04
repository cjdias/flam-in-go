package watchdog

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.watchdog"

// KennelId is the id of the [flam.IKennel] provided by the bundle
// returned from [NewBundle].
const KennelId = "flam.watchdog.kennel"

// Configuration paths and container groups of the logger adaptors and the
// processes.
const (
	// LoggerConfigPath is the configuration path holding one logger
	// adaptor configuration per adaptor id, as read by the logger adaptor
	// factory.
	LoggerConfigPath = "flam.watchdog.loggers"

	// LoggerCreatorGroup is the container group the logger adaptor driver
	// bundles provide their [flam.IWatchdogLoggerAdaptorCreator] in,
	// collected by the logger adaptor factory.
	LoggerCreatorGroup = "flam.watchdog.loggers.creator"

	// ProcessGroup is the container group the application provides its
	// [flam.IWatchdogProcess] values in, added to the kennel when it is
	// created.
	ProcessGroup = "flam.watchdog.processes"

	// ProcessConfigPath is the configuration path holding the
	// configuration of the processes of [ProcessGroup], by process id,
	// with the [ConfigFieldActive] and [ConfigFieldLogger] fields.
	ProcessConfigPath = "flam.watchdog.processes"

	// RunConfigPath is the configuration path of a bool telling whether
	// the watchdog bundle, when the application runs, runs the active
	// processes of the kennel. It defaults to true.
	RunConfigPath = "flam.watchdog.run"

	// SignalsConfigPath is the configuration path of a bool telling
	// whether the kennel, while it runs its processes, closes itself, and
	// so its processes, on an interrupt or a SIGTERM signal, so that the
	// run of the application ends, a second signal ending the program as
	// usual. It defaults to true.
	SignalsConfigPath = "flam.watchdog.signals"

	// RestartDelayConfigPath is the configuration path of the delay before
	// the first restart of a process that panicked, a duration, doubled at
	// each following restart, and reset after a run lasting the maximum
	// delay at least, such as "100ms". It defaults to 100ms, 0 restarting
	// at once the first time, the following delays then doubling from
	// 10ms.
	RestartDelayConfigPath = "flam.watchdog.restart_delay"

	// RestartMaxDelayConfigPath is the configuration path of the maximum
	// delay before the restart of a process that panicked, a duration not
	// shorter than the restart delay. It defaults to 30s, or to the restart
	// delay when longer, 0 restarting a process at once every time.
	RestartMaxDelayConfigPath = "flam.watchdog.restart_max_delay"
)

// Fields of a process configuration, at [ProcessConfigPath].
const (
	// ConfigFieldActive is the field holding whether the process is
	// active, a bool, true unless set.
	ConfigFieldActive = "active"

	// ConfigFieldLogger is the field holding the id of the logger adaptor
	// of the process, in the logger adaptor factory, an empty id leaving
	// the process without logger adaptor.
	ConfigFieldLogger = "logger"
)

// Configuration paths of the values taken by the fields missing from a
// process configuration.
const (
	// DefaultLoggerConfigPath is the path of the default of
	// [ConfigFieldLogger], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers/standard] to
	// its adaptor, and unset otherwise: the processes then have no logger
	// adaptor.
	DefaultLoggerConfigPath = "flam.watchdog.defaults.processes.logger"
)

// Context fields of the [*flam.Error] values returned by the kennel.
const (
	// ErrorFieldID holds the id of a process, or [KennelId] for an error
	// of the kennel itself, such as an invalid restart delay.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field, or
	// the path of an invalid process configuration or restart delay.
	ErrorFieldField = "field"

	// ErrorFieldLogger holds the id of the logger adaptor of a process.
	ErrorFieldLogger = "logger"

	// ErrorFieldProcess holds the id of a process added under another id.
	ErrorFieldProcess = "process"

	// ErrorFieldPanic holds the value a process panicked with.
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack of the goroutine of a process when
	// it panicked.
	ErrorFieldStack = "stack"
)

// Defaults of the watchdog bundle behaviour and of the process fields.
const (
	defaultRun             = true
	defaultSignals         = true
	defaultActive          = true
	defaultRestartDelay    = 100 * time.Millisecond
	defaultRestartMaxDelay = 30 * time.Second

	// minRestartDelay is the least delay of a restart following another
	// one, doubled from a delay of 0.
	minRestartDelay = 10 * time.Millisecond
)
