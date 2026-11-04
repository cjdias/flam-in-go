package log

import (
	"errors"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrMissingTriggerFactory is the error wrapped by the error returned
	// from the Boot method of the bundle returned by [NewBundle] when the
	// period at [FlusherPeriodConfigPath] is positive and the container
	// provides no [github.com/cjdias/flam-in-go/pkg/flam.ITriggerFactory].
	ErrMissingTriggerFactory = errors.New("trigger factory not provided by the container")

	// ErrInvalidLevel is the error wrapped by the error returned from the
	// SetLevel method of the streams of the drivers when the level is not
	// between [github.com/cjdias/flam-in-go/pkg/flam.LogNone] and
	// [github.com/cjdias/flam-in-go/pkg/flam.LogDebug].
	ErrInvalidLevel = errors.New("invalid log level")

	// ErrInvalidChannel is the error wrapped by the error returned from the
	// AddChannel method of the streams of the drivers when the channel is
	// empty.
	ErrInvalidChannel = errors.New("invalid log channel")

	// ErrStreamClosed is the error wrapped by the error returned from the
	// Signal and Broadcast methods of the streams of the drivers once
	// closed.
	ErrStreamClosed = errors.New("log stream closed")
)

// newErrBoot returns the error wrapping the error of the creation of a
// stream when the application boots, with the message and the
// ErrorFieldID context field.
func newErrBoot(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}

// newErrStream returns the error wrapping the error of a stream the logger
// writes to with the message and the ErrorFieldID context field.
func newErrStream(
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

// newErrFlusher returns the error wrapping the error of the observer
// registration, of the trigger creation or of the final flush of the
// flusher with the message.
func newErrFlusher(
	message string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message)
}
