package pubsub

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrClosed is the error wrapped by the errors returned from
	// [PubSub.Subscribe], [PubSub.Unsubscribe] and [PubSub.Publish] once
	// the PubSub is closed.
	ErrClosed = errors.New("pubsub closed")

	// ErrInvalidPattern is the error wrapped by the error returned
	// from [PubSub.Subscribe] when the pattern is malformed.
	ErrInvalidPattern = errors.New("invalid pubsub pattern")

	// ErrInvalidTopic is the error wrapped by the error returned from
	// [PubSub.Publish] when the topic is malformed.
	ErrInvalidTopic = errors.New("invalid pubsub topic")

	// ErrNilHandler is the error wrapped by the error returned from
	// [PubSub.Subscribe] when the handler is nil.
	ErrNilHandler = errors.New("nil pubsub handler")

	// ErrDuplicateID is the error wrapped by the error returned from
	// [PubSub.Subscribe] when the id is already used by a subscription.
	ErrDuplicateID = errors.New("pubsub subscription id already in use")

	// ErrUnknownID is the error wrapped by the error returned from
	// [PubSub.Unsubscribe] when no subscription uses the id.
	ErrUnknownID = errors.New("unknown pubsub subscription id")

	// ErrInvalidID is the error wrapped by the errors returned from
	// [PubSub.Subscribe] and [PubSub.Unsubscribe] when the id cannot be
	// used as a map key: a value of an interface ID type, such as any,
	// holding a slice, a map, a function or a struct containing one.
	ErrInvalidID = errors.New("invalid pubsub subscription id")

	// ErrHandlerPanic is the error wrapped by the error passed to the
	// error callback of a subscription whose handler panicked.
	ErrHandlerPanic = errors.New("pubsub handler panicked")

	// ErrHandlerExit is the error wrapped by the error passed to the
	// error callback of a subscription whose handler ended its goroutine
	// with [runtime.Goexit], as [testing.T.FailNow] does.
	ErrHandlerExit = errors.New("pubsub handler exited")

	// ErrCloseFromHandler is the error wrapped by the error returned
	// from [PubSub.Close] when called from a handler or an error callback of
	// the same PubSub, which Close would otherwise wait for forever.
	ErrCloseFromHandler = errors.New("pubsub closed from one of its handlers")
)

// newErrClosed returns the error wrapping ErrClosed with the message.
func newErrClosed(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrClosed, message)
}

// newErrInvalidPattern returns the error wrapping ErrInvalidPattern with
// the message and the ErrorFieldID and ErrorFieldPattern context fields.
func newErrInvalidPattern(
	message string,
	id any,
	pattern string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidPattern, message).
		With(ErrorFieldID, id).
		With(ErrorFieldPattern, pattern)
}

// newErrInvalidTopic returns the error wrapping ErrInvalidTopic with the
// message and the ErrorFieldTopic context field.
func newErrInvalidTopic(
	message string,
	topic string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidTopic, message).
		With(ErrorFieldTopic, topic)
}

// newErrNilHandler returns the error wrapping ErrNilHandler with the
// message and the ErrorFieldID and ErrorFieldPattern context fields.
func newErrNilHandler(
	message string,
	id any,
	pattern string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilHandler, message).
		With(ErrorFieldID, id).
		With(ErrorFieldPattern, pattern)
}

// newErrDuplicateID returns the error wrapping ErrDuplicateID with the
// message and the ErrorFieldID and ErrorFieldPattern context fields.
func newErrDuplicateID(
	message string,
	id any,
	pattern string,
) *flam.Error {
	return flam.NewWrappedError(ErrDuplicateID, message).
		With(ErrorFieldID, id).
		With(ErrorFieldPattern, pattern)
}

// newErrUnknownID returns the error wrapping ErrUnknownID with the message
// and the ErrorFieldID context field.
func newErrUnknownID(
	message string,
	id any,
) *flam.Error {
	return flam.NewWrappedError(ErrUnknownID, message).
		With(ErrorFieldID, id)
}

// newErrInvalidID returns the error wrapping ErrInvalidID with the message
// and the ErrorFieldID context field.
func newErrInvalidID(
	message string,
	id any,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidID, message).
		With(ErrorFieldID, id)
}

// newErrHandlerPanic returns the error wrapping ErrHandlerPanic, joined
// with the panic value when it is an error, with the message and the
// ErrorFieldID, ErrorFieldTopic, ErrorFieldPanic and ErrorFieldStack
// context fields.
func newErrHandlerPanic(
	message string,
	id any,
	topic string,
	value any,
	stack []byte,
) *flam.Error {
	cause := ErrHandlerPanic
	if panicErr, isErr := value.(error); isErr {
		cause = fmt.Errorf("%w: %w", ErrHandlerPanic, panicErr)
	}
	return flam.NewWrappedError(cause, message).
		With(ErrorFieldID, id).
		With(ErrorFieldTopic, topic).
		With(ErrorFieldPanic, value).
		With(ErrorFieldStack, string(stack))
}

// newErrHandlerExit returns the error wrapping ErrHandlerExit with the
// message and the ErrorFieldID and ErrorFieldTopic context fields.
func newErrHandlerExit(
	message string,
	id any,
	topic string,
) *flam.Error {
	return flam.NewWrappedError(ErrHandlerExit, message).
		With(ErrorFieldID, id).
		With(ErrorFieldTopic, topic)
}

// newErrCloseFromHandler returns the error wrapping ErrCloseFromHandler
// with the message.
func newErrCloseFromHandler(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrCloseFromHandler, message)
}

// newErrContextDone returns the error wrapping cause with the message and
// the ErrorFieldTopic context field.
func newErrContextDone(
	message string,
	cause error,
	topic string,
) *flam.Error {
	return flam.NewWrappedError(cause, message).
		With(ErrorFieldTopic, topic)
}
