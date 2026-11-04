package funit

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilApp is the error wrapped by the error returned from
	// [NewRunner] when the application is nil.
	ErrNilApp = errors.New("nil funit application")

	// ErrNilTest is the error wrapped by the error returned from
	// [NewRunner] when the test is nil.
	ErrNilTest = errors.New("nil funit test")

	// ErrInvalidStage is the error wrapped by the error returned from Run
	// when a builder was given a stage function that is not a function, or
	// an empty id, held by [ErrorFieldStage].
	ErrInvalidStage = errors.New("invalid funit stage")

	// ErrRan is the error wrapped by the error returned from Run when the
	// runner ran already, as a run ends the processes of the application.
	ErrRan = errors.New("funit runner ran already")

	// ErrConfig is the error wrapped, along with its cause, by the error
	// returned from Run when a configuration file or values cannot be
	// added to the configuration of the application.
	ErrConfig = errors.New("unable to add funit configuration")

	// ErrProcess is the error wrapped, along with its cause, by the error
	// returned from Run when a watchdog process of the test cannot be
	// activated, such as without the watchdog bundle.
	ErrProcess = errors.New("unable to activate funit process")

	// ErrBoot is the error wrapped, along with the error of the
	// application, by the error returned from Run when the application
	// fails to boot.
	ErrBoot = errors.New("unable to boot funit application")

	// ErrStage is the error wrapped, along with its cause, by the error
	// returned from Run when a stage function fails, or cannot be invoked.
	ErrStage = errors.New("funit stage failed")

	// ErrRun is the error wrapped, along with the error of the
	// application, by the error returned from Run when the run of the
	// application fails.
	ErrRun = errors.New("funit application run failed")

	// ErrStartTimeout is the error wrapped by the error returned from Run
	// when the processes of the test do not run within the start timeout.
	ErrStartTimeout = errors.New("funit processes did not start in time")

	// ErrStopTimeout is the error wrapped by the error returned from Run
	// when the run of the application does not end within the stop timeout
	// once its processes are closed.
	ErrStopTimeout = errors.New("funit application did not stop in time")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message.
func newErr(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message)
}
