package funit

import (
	"testing"
)

// Logger reports the progress of the stages of a [Runner], set with
// WithLogger. The runner reports nothing unless given one, such as the
// one returned by [NewLogger].
type Logger interface {
	// Stage reports that the stage, such as "arrange", starts.
	Stage(stage string)

	// Step reports that the function of the id, of the stage, runs.
	Step(stage, id string)

	// Error reports that the function of the id, of the stage, or the
	// stage itself when the id is empty, failed with e.
	Error(stage, id string, e error)
}

// NewLogger returns the [Logger] reporting the progress of a runner in the
// log of the test, shown when it fails, or run with -v, or nothing for a
// nil test.
func NewLogger(
	t testing.TB,
) Logger {
	if isNil(t) {
		return &logger{}
	}
	return &logger{t: t}
}

// Names of the stages reported to the logger.
const (
	StageConfig   = "config"
	StageBoot     = "boot"
	StageArrange  = "arrange"
	StageRun      = "run"
	StageAct      = "act"
	StageAssert   = "assert"
	StageStop     = "stop"
	StageTeardown = "teardown"
)

// logger is the [Logger] returned by [NewLogger], reporting nothing when
// it has no test, as the logger of a runner given none.
type logger struct {
	t testing.TB
}

func (l *logger) Stage(
	stage string,
) {
	if l.t == nil {
		return
	}
	l.t.Helper()
	l.t.Logf("funit: %s", stage)
}

func (l *logger) Step(
	stage string,
	id string,
) {
	if l.t == nil {
		return
	}
	l.t.Helper()
	l.t.Logf("funit: %s: %s", stage, id)
}

func (l *logger) Error(
	stage string,
	id string,
	e error,
) {
	switch {
	case l.t == nil:
		return
	case id == "":
		l.t.Helper()
		l.t.Logf("funit: %s failed: %v", stage, e)
	default:
		l.t.Helper()
		l.t.Logf("funit: %s: %s failed: %v", stage, id, e)
	}
}
