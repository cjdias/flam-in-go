package broker

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrClosed is the error wrapped by the errors returned from the
	// Publish and Subscribe methods of a closed broker.
	ErrClosed = errors.New("broker closed")

	// ErrInvalidTopic is the error wrapped by the errors returned from the
	// Publish and Subscribe methods of a broker when the topic is empty.
	ErrInvalidTopic = errors.New("invalid broker topic")

	// ErrNilHandler is the error wrapped by the errors returned from the
	// Subscribe method of a broker, [SubscribeJSON] and [NewProcess] when
	// the handler is nil.
	ErrNilHandler = errors.New("nil broker handler")

	// ErrNilBroker is the error wrapped by the errors returned from
	// [PublishJSON], [SubscribeJSON] and [NewProcess] when the broker is
	// nil.
	ErrNilBroker = errors.New("nil broker")

	// ErrPublish is the error wrapped, along with the error of the
	// backend, by the error returned from the Publish method of a broker
	// when the backend refuses the message.
	ErrPublish = errors.New("unable to publish broker message")

	// ErrSubscribe is the error wrapped, along with the error of the
	// backend, by the error returned from the Subscribe method of a broker
	// when the backend refuses the subscription.
	ErrSubscribe = errors.New("unable to subscribe to broker topic")

	// ErrReceive is the error wrapped, along with the error of the
	// backend, by the error ending a subscription, returned by its Err
	// method, when receiving its messages fails.
	ErrReceive = errors.New("unable to receive broker message")

	// ErrHandlerPanic is the error wrapped, joined with the panic value
	// when it is an error, by the error a broker driver takes as returned
	// by a handler that panicked, which leaves the message
	// unacknowledged.
	ErrHandlerPanic = errors.New("broker handler panicked")

	// ErrEncode is the error wrapped, along with the error of the encoder,
	// by the error returned from [PublishJSON] when the value cannot be
	// encoded.
	ErrEncode = errors.New("unable to encode broker message")

	// ErrDecode is the error wrapped, along with the error of the decoder,
	// by the error the handler of [SubscribeJSON] returns, without calling
	// the typed handler, when a payload cannot be decoded.
	ErrDecode = errors.New("unable to decode broker message")

	// ErrRunning is the error wrapped by the error returned from the Run
	// method of a process of [NewProcess] when it runs already.
	ErrRunning = errors.New("broker process running")
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
