package broker

import (
	"context"
	"encoding/json"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// PublishJSON publishes the JSON encoding of the value on the topic of the
// broker, with the headers, which may be nil. It fails with ErrNilBroker
// when the broker is nil, ErrEncode, holding the topic, when the value
// cannot be encoded, or the error of the Publish method of the broker.
func PublishJSON(
	ctx context.Context,
	b flam.IBroker,
	topic string,
	value any,
	headers map[string]string,
) error {
	if b == nil {
		return newErr(ErrNilBroker, publishJSONMessage, nil).
			With(ErrorFieldTopic, topic)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return newErr(ErrEncode, publishJSONMessage, err).
			With(ErrorFieldTopic, topic)
	}
	return b.Publish(ctx, topic, payload, headers)
}

// SubscribeJSON subscribes to the topic of the broker the handler of the
// values of type T decoded from the JSON payloads of the messages, as the
// Subscribe method of the broker does. A payload that cannot be decoded
// into a T is not given to the handler: the subscription takes it as
// failing with the error wrapping ErrDecode, holding the topic and the id
// of the message, so that a driver delivering the failed messages again
// delivers it again. SubscribeJSON fails with ErrNilBroker or
// ErrNilHandler, holding the topic, or the error of the Subscribe method
// of the broker.
func SubscribeJSON[T any](
	ctx context.Context,
	b flam.IBroker,
	topic string,
	handler func(ctx context.Context, msg flam.IBrokerMessage, value T) error,
) (flam.IBrokerSubscription, error) {
	switch {
	case b == nil:
		return nil, newErr(ErrNilBroker, subscribeJSONMessage, nil).
			With(ErrorFieldTopic, topic)
	case handler == nil:
		return nil, newErr(ErrNilHandler, subscribeJSONMessage, nil).
			With(ErrorFieldTopic, topic)
	}
	return b.Subscribe(ctx, topic, func(ctx context.Context, msg flam.IBrokerMessage) error {
		var value T
		if err := json.Unmarshal(msg.Payload(), &value); err != nil {
			return newErr(ErrDecode, subscribeJSONMessage, err).
				With(ErrorFieldTopic, msg.Topic()).
				With(ErrorFieldMessage, msg.Id())
		}
		return handler(ctx, msg, value)
	})
}

// Messages of the errors returned by PublishJSON and SubscribeJSON.
const (
	publishJSONMessage   = "unable to publish broker json message"
	subscribeJSONMessage = "unable to subscribe to broker json messages"
)
