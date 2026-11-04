package config

import (
	"errors"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

var (
	// ErrInvalidTarget is the error wrapped by the errors returned
	// from [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// Populate and PopulatePath when the target is not a non-nil pointer.
	ErrInvalidTarget = errors.New("invalid config populate target")

	// ErrMissingPath is the error wrapped by the error returned from
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// PopulatePath when no value is stored at the path.
	ErrMissingPath = errors.New("config path not found")

	// ErrNilObserver is the error wrapped by the error returned from
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// AddObserver when the callback is nil.
	ErrNilObserver = errors.New("nil config observer")

	// ErrDuplicateObserver is the error wrapped by the error returned
	// from [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// AddObserver when an observer already uses the id, whatever its path.
	ErrDuplicateObserver = errors.New("config observer id already in use")

	// ErrUnknownObserver is the error wrapped by the error returned
	// from [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// RemoveObserver when no observer uses the id.
	ErrUnknownObserver = errors.New("unknown config observer id")

	// ErrNilSource is the error wrapped by the error returned from
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// AddSource when the source is nil.
	ErrNilSource = errors.New("nil config source")

	// ErrDuplicateSource is the error wrapped by the error returned from
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// AddSource when a source with the same id was added.
	ErrDuplicateSource = errors.New("config source id already in use")

	// ErrUnknownSource is the error wrapped by the error returned from
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
	// RemoveSource when no source has the id.
	ErrUnknownSource = errors.New("unknown config source id")

	// ErrMissingTriggerFactory is the error wrapped by the error returned
	// from the Boot method of the bundle returned by [NewBundle] when the
	// period at [ReloadPeriodConfigPath] is positive and the container
	// provides no [github.com/cjdias/flam-in-go/pkg/flam.ITriggerFactory].
	ErrMissingTriggerFactory = errors.New("trigger factory not provided by the container")
)

// newErrInvalidTarget returns the error wrapping ErrInvalidTarget with the
// message.
func newErrInvalidTarget(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidTarget, message)
}

// newErrMissingPath returns the error wrapping ErrMissingPath with the
// message and the ErrorFieldPath context field.
func newErrMissingPath(
	message string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrMissingPath, message).
		With(ErrorFieldPath, path)
}

// newErrDecode returns the error wrapping the error of encoding/json with
// the message.
func newErrDecode(
	message string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message)
}

// newErrNilObserver returns the error wrapping ErrNilObserver with the
// message and the ErrorFieldID and ErrorFieldPath context fields.
func newErrNilObserver(
	message string,
	id string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilObserver, message).
		With(ErrorFieldID, id).
		With(ErrorFieldPath, path)
}

// newErrObserverPath returns the error wrapping
// [container.ErrInvalidPath] with the message and the ErrorFieldID and
// ErrorFieldPath context fields.
func newErrObserverPath(
	message string,
	id string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(container.ErrInvalidPath, message).
		With(ErrorFieldID, id).
		With(ErrorFieldPath, path)
}

// newErrDuplicateObserver returns the error wrapping ErrDuplicateObserver
// with the message and the ErrorFieldID context field.
func newErrDuplicateObserver(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrDuplicateObserver, message).
		With(ErrorFieldID, id)
}

// newErrUnknownObserver returns the error wrapping ErrUnknownObserver with
// the message and the ErrorFieldID context field.
func newErrUnknownObserver(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrUnknownObserver, message).
		With(ErrorFieldID, id)
}

// newErrNilSource returns the error wrapping ErrNilSource with the message.
func newErrNilSource(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilSource, message)
}

// newErrDuplicateSource returns the error wrapping ErrDuplicateSource with
// the message and the ErrorFieldID context field.
func newErrDuplicateSource(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrDuplicateSource, message).
		With(ErrorFieldID, id)
}

// newErrUnknownSource returns the error wrapping ErrUnknownSource with the
// message and the ErrorFieldID context field.
func newErrUnknownSource(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrUnknownSource, message).
		With(ErrorFieldID, id)
}

// newErrSourceValues returns the error wrapping the error of the bag merge
// of the values of a source with the message and the ErrorFieldID context
// field.
func newErrSourceValues(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}

// newErrReload returns the error wrapping the error of the Reload method
// of a source with the message and the ErrorFieldID context field.
func newErrReload(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}

// newErrBoot returns the error wrapping the error of the creation or the
// addition of a source when the application boots, with the message and
// the ErrorFieldID context field.
func newErrBoot(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}

// newErrMissingTriggerFactory returns the error wrapping
// ErrMissingTriggerFactory with the message and the ErrorFieldPeriod
// context field.
func newErrMissingTriggerFactory(
	message string,
	period time.Duration,
) *flam.Error {
	return flam.NewWrappedError(ErrMissingTriggerFactory, message).
		With(ErrorFieldPeriod, period)
}

// newErrReloader returns the error wrapping the error of the observer
// registration or of the trigger creation of the reloader with the
// message.
func newErrReloader(
	message string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message)
}
