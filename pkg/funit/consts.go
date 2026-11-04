package funit

import (
	"time"
)

// Defaults of the runner.
const (
	// DefaultStartTimeout is the time the runner waits for the application
	// to run the processes of the test, unless set by WithStartTimeout.
	DefaultStartTimeout = 10 * time.Second

	// DefaultStopTimeout is the time the runner waits for the run of the
	// application to end once its processes are closed, unless set by
	// WithStopTimeout.
	DefaultStopTimeout = 10 * time.Second

	// DefaultPublishedTimeout is the time the assertion of
	// WithAssertPublished waits for the messages to be delivered, as the
	// pub/sub delivers them on goroutines of their own, unless set by
	// WithPublishedTimeout.
	DefaultPublishedTimeout = time.Second

	// DefaultConfigParser is the id of the config parser reading the files
	// of WithConfig, unless another is given.
	DefaultConfigParser = "yaml"
)

// Context fields of the [*flam.Error] values returned by the runner.
const (
	// ErrorFieldStage holds the id of the stage function, or the name of
	// the builder, the error is about.
	ErrorFieldStage = "stage"

	// ErrorFieldFile holds the path of a configuration file.
	ErrorFieldFile = "file"

	// ErrorFieldParser holds the id of a configuration parser.
	ErrorFieldParser = "parser"

	// ErrorFieldProcess holds the id of a watchdog process.
	ErrorFieldProcess = "process"

	// ErrorFieldTimeout holds the duration of the timeout that elapsed.
	ErrorFieldTimeout = "timeout"
)

// pollInterval is the time between two checks of the processes of the
// application while waiting for them to run.
const pollInterval = time.Millisecond
