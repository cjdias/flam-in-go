package invalidating

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewAdaptor returns the [flam.ICacheKeyedAdaptor] of the id wrapping the
// wrapped adaptor, a [flam.ICacheKeyedAdaptor]: it reads and writes the
// values in the wrapped adaptor, then publishes the keys it removes, and
// those it stores unless publishSet is false, on the topic of the broker,
// while removing from the wrapped adaptor the keys the other adaptors
// publish there, ignoring its own messages.
//
// It subscribes to the topic before returning, the subscription lasting
// until the adaptor is closed, and fails with ErrSubscribe, joined with
// the error of the broker, when it cannot. It fails with ErrNilAdaptor or
// ErrNilBroker when the wrapped adaptor or the broker is nil, with
// ErrNotKeyed when the wrapped adaptor is not a
// [flam.ICacheKeyedAdaptor], and with ErrInvalidField when the topic is
// empty.
//
// Closing the adaptor closes its subscription, but neither the wrapped
// adaptor nor the broker, owned by their factories.
func NewAdaptor(
	id string,
	wrapped flam.ICacheAdaptor,
	broker flam.IBroker,
	topic string,
	publishSet bool,
) (flam.ICacheKeyedAdaptor, error) {
	if value.IsNil(wrapped) {
		return nil, newErrNil(ErrNilAdaptor, adaptorNewMessage, id)
	}
	keyed, ok := wrapped.(flam.ICacheKeyedAdaptor)
	switch {
	case !ok:
		return nil, newErrNil(ErrNotKeyed, adaptorNewMessage, id).
			With(ErrorFieldAdaptor, wrapped.Id())
	case value.IsNil(broker):
		return nil, newErrNil(ErrNilBroker, adaptorNewMessage, id)
	case topic == "":
		return nil, newErrInvalidField(adaptorNewMessage, id, ConfigFieldTopic)
	}

	a := &adaptor{
		id:         id,
		wrapped:    keyed,
		broker:     broker,
		topic:      topic,
		publishSet: publishSet,
		instance:   rand.Text(),
	}
	sub, err := broker.Subscribe(context.Background(), topic, a.handle)
	if err != nil {
		return nil, newErrJoined(ErrSubscribe, adaptorNewMessage, id, ErrorFieldTopic, topic, err)
	}
	a.subscription = sub
	return a, nil
}

// adaptor is the [flam.ICacheKeyedAdaptor] returned by
// [NewAdaptor].
type adaptor struct {
	id           string
	wrapped      flam.ICacheKeyedAdaptor
	broker       flam.IBroker
	topic        string
	publishSet   bool
	instance     string // the random id of the adaptor, in HeaderInstance
	subscription flam.IBrokerSubscription

	closeOnce sync.Once
	closeErr  error
}

func (a *adaptor) Id() string {
	return a.id
}

// Close closes the subscription, once, returning its error.
func (a *adaptor) Close() error {
	a.closeOnce.Do(func() {
		a.closeErr = a.subscription.Close()
	})
	return a.closeErr
}

// Has calls the Has method of the adaptor wrapped.
func (a *adaptor) Has(
	ctx context.Context,
	objects ...any,
) ([]bool, error) {
	return a.wrapped.Has(ctx, objects...)
}

// Get calls the Get method of the adaptor wrapped.
func (a *adaptor) Get(
	ctx context.Context,
	objects ...any,
) ([]any, error) {
	return a.wrapped.Get(ctx, objects...)
}

// Set stores the objects in the adaptor wrapped, then publishes their
// keys, unless publishSet is false.
func (a *adaptor) Set(
	ctx context.Context,
	objects ...any,
) error {
	return a.set(ctx, objects, func() error {
		return a.wrapped.Set(ctx, objects...)
	})
}

// SetEphemeral stores the objects in the adaptor wrapped, then publishes
// their keys, unless publishSet is false.
func (a *adaptor) SetEphemeral(
	ctx context.Context,
	ttl time.Duration,
	objects ...any,
) error {
	return a.set(ctx, objects, func() error {
		return a.wrapped.SetEphemeral(ctx, ttl, objects...)
	})
}

// Del removes the values of the objects from the adaptor wrapped, then
// publishes their keys.
func (a *adaptor) Del(
	ctx context.Context,
	objects ...any,
) error {
	keys, err := a.wrapped.Keys(objects...)
	if err != nil {
		return err
	}
	return a.DelKeys(ctx, keys...)
}

// Keys calls the Keys method of the adaptor wrapped.
func (a *adaptor) Keys(
	objects ...any,
) ([]string, error) {
	return a.wrapped.Keys(objects...)
}

// DelKeys removes the values of the keys from the adaptor wrapped, then
// publishes them.
func (a *adaptor) DelKeys(
	ctx context.Context,
	keys ...string,
) error {
	if err := a.wrapped.DelKeys(ctx, keys...); err != nil {
		return err
	}
	return a.publish(ctx, adaptorDelMessage, keys)
}

// Messages of the errors returned by NewAdaptor, the adaptor methods and
// the handler of the subscription.
const (
	adaptorNewMessage    = "unable to create invalidating cache adaptor"
	adaptorSetMessage    = "unable to set in invalidating cache"
	adaptorDelMessage    = "unable to delete from invalidating cache"
	adaptorHandleMessage = "unable to handle invalidating cache keys"
)

// set takes the keys of the objects when publishing them, then stores
// the objects with the store function and publishes their keys.
func (a *adaptor) set(
	ctx context.Context,
	objects []any,
	store func() error,
) error {
	if !a.publishSet {
		return store()
	}
	keys, err := a.wrapped.Keys(objects...)
	if err != nil {
		return err
	}
	if err := store(); err != nil {
		return err
	}
	return a.publish(ctx, adaptorSetMessage, keys)
}

// publish publishes the keys, if any, as a JSON array, with the instance
// of the adaptor in the HeaderInstance header.
func (a *adaptor) publish(
	ctx context.Context,
	message string,
	keys []string,
) error {
	if len(keys) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, _ := json.Marshal(keys) // a []string always encodes
	headers := map[string]string{HeaderInstance: a.instance}
	if err := a.broker.Publish(ctx, a.topic, payload, headers); err != nil {
		return newErrJoined(ErrPublish, message, a.id, ErrorFieldTopic, a.topic, err)
	}
	return nil
}

// handle removes from the adaptor wrapped the keys of a message published
// by another adaptor, ignoring the messages of this one.
func (a *adaptor) handle(
	ctx context.Context,
	msg flam.IBrokerMessage,
) error {
	if msg.Headers()[HeaderInstance] == a.instance {
		return nil
	}
	var keys []string
	if err := json.Unmarshal(msg.Payload(), &keys); err != nil {
		return newErrJoined(ErrDecode, adaptorHandleMessage, a.id, ErrorFieldTopic, msg.Topic(), err)
	}
	return a.wrapped.DelKeys(ctx, keys...)
}
