package broker

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the check configuration, or its default,
	// does not hold a value of the expected type: a kind read by
	// [github.com/cjdias/flam-in-go/pkg/flam/health.ParseKind], a
	// non-empty broker id and a topic.
	ErrInvalidField = errors.New("invalid broker health check field")

	// ErrNoFactory is the error wrapped by the error returned from the
	// creator when the container provides no [flam.IBrokerFactory], as
	// the bundle of [github.com/cjdias/flam-in-go/pkg/flam/broker] is not
	// registered.
	ErrNoFactory = errors.New("no broker factory")

	// ErrUnknownBroker is the error wrapped by the error returned from the
	// creator when the broker factory has no broker of the id.
	ErrUnknownBroker = errors.New("unknown broker")

	// ErrBroker is the error wrapped, along with the error of the broker
	// factory, by the error of a check when the broker cannot be taken.
	ErrBroker = errors.New("unable to get broker")

	// ErrPing is the error wrapped, along with the error of the ping, by
	// the error of a check when the broker, implementing
	// [flam.IHealthPinger], tells its backend does not answer.
	ErrPing = errors.New("unable to ping broker")

	// ErrPublish is the error wrapped, along with the error of the
	// publication, by the error of a check when the message cannot be
	// published on its topic.
	ErrPublish = errors.New("unable to publish on broker")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the ErrorFieldID context field.
func newErr(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}
