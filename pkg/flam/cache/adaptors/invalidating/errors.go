package invalidating

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the adaptor configuration, or its default,
	// does not hold a value of the expected type: non-empty adaptor and
	// broker ids, the adaptor id being another one than the adaptor
	// itself, a non-empty topic and a bool publish set. [NewAdaptor] also
	// returns it for an empty topic.
	ErrInvalidField = errors.New("invalid invalidating cache adaptor field")

	// ErrAdaptor is the error wrapped, along with the error of the
	// [flam.ICacheAdaptorFactory], by the error returned from the creator
	// when the adaptor wrapped cannot be taken from the factory.
	ErrAdaptor = errors.New("unable to get invalidating cache adaptor wrapped adaptor")

	// ErrBroker is the error wrapped, along with the error of the
	// [flam.IBrokerFactory], by the error returned from the creator when
	// the broker cannot be taken from the factory, or when no bundle
	// provides the factory.
	ErrBroker = errors.New("unable to get invalidating cache adaptor broker")

	// ErrNilAdaptor and ErrNilBroker are the errors wrapped by the error
	// returned from [NewAdaptor] when the adaptor wrapped or the broker is
	// nil.
	ErrNilAdaptor = errors.New("nil invalidating cache adaptor wrapped adaptor")
	ErrNilBroker  = errors.New("nil invalidating cache adaptor broker")

	// ErrNotKeyed is the error wrapped by the error returned from
	// [NewAdaptor], and so from the creator, when the adaptor wrapped is
	// not a [flam.ICacheKeyedAdaptor].
	ErrNotKeyed = errors.New("invalidating cache adaptor wrapped adaptor tells no keys")

	// ErrSubscribe is the error wrapped, along with the error of the
	// broker, by the error returned from [NewAdaptor] when it cannot
	// subscribe to the topic.
	ErrSubscribe = errors.New("unable to subscribe invalidating cache adaptor")

	// ErrPublish is the error wrapped, along with the error of the broker,
	// by the error returned from the adaptor methods removing or storing
	// values when the keys cannot be published, the values being removed
	// or stored in the adaptor wrapped already.
	ErrPublish = errors.New("unable to publish invalidating cache adaptor keys")

	// ErrDecode is the error wrapped, along with the error of the JSON
	// decoding, by the error the handler of the subscription returns for a
	// message whose payload is not a JSON array of keys.
	ErrDecode = errors.New("unable to decode invalidating cache adaptor keys")
)

// errNoFactory and errNoBrokers are the causes of the ErrAdaptor and
// ErrBroker errors when the creator was given no adaptor factory, or no
// bundle provides the broker factory.
var (
	errNoFactory = errors.New("no cache adaptor factory")
	errNoBrokers = errors.New("no broker factory")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrJoined returns the error wrapping the sentinel, joined with the
// error of a factory or of the broker, with the message and the
// ErrorFieldID context field and the field of the value it is about.
func newErrJoined(
	sentinel error,
	message string,
	id string,
	field string,
	value string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, err), message).
		With(ErrorFieldID, id).
		With(field, value)
}

// newErrNil returns the error wrapping the sentinel of a nil or invalid
// argument, with the message and the ErrorFieldID context field.
func newErrNil(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}
