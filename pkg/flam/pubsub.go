package flam

import (
	"context"
)

// IPubSub is the interface of a
// [github.com/cjdias/flam-in-go/pkg/flam/pubsub.PubSub], for code that
// publishes or subscribes to messages of type DATA, with subscription ids
// of type ID, without depending on the PubSub type itself, so that tests
// can fake it.
//
// *pubsub.PubSub implements IPubSub, and each method behaves as the PubSub
// method of the same name, whose documentation describes topics, patterns,
// delivery, the lifecycle and errors in full.
type IPubSub[ID comparable, DATA any] interface {
	// Close stops every subscription and waits until every queued message
	// has been handled, or skipped because its context is done. See
	// [github.com/cjdias/flam-in-go/pkg/flam/pubsub.PubSub.Close].
	Close() error

	// Subscribe registers a subscription, identified by id, delivering to
	// handler the messages published on the topics matching pattern, and
	// passing delivery failures to onError, which may be nil. See
	// [github.com/cjdias/flam-in-go/pkg/flam/pubsub.PubSub.Subscribe].
	Subscribe(
		id ID,
		pattern string,
		handler func(ctx context.Context, topic string, msg DATA) error,
		onError func(topic string, msg DATA, err error),
	) error

	// Unsubscribe removes the subscription identified by id, which still
	// handles the messages already queued. See
	// [github.com/cjdias/flam-in-go/pkg/flam/pubsub.PubSub.Unsubscribe].
	Unsubscribe(id ID) error

	// Publish queues msg for every subscription whose pattern matches
	// topic, without waiting for the handlers. See
	// [github.com/cjdias/flam-in-go/pkg/flam/pubsub.PubSub.Publish].
	Publish(ctx context.Context, topic string, msg DATA) error
}
