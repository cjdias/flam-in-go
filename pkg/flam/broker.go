package flam

import (
	"context"
	"io"

	"github.com/segmentio/kafka-go"
)

// IBrokerMessage is a message received from an [IBroker], handed to the
// [BrokerHandler] of a subscription. Its payload and headers are those
// given to the Publish method of the broker that sent it. It may be read
// from several goroutines, but must not be changed.
type IBrokerMessage interface {
	// Id returns the id the backend gave the message, such as the entry id
	// of a Redis stream, the sequence of a JetStream message or the
	// partition and offset of a Kafka message, or "" when the backend
	// gives none, as Redis Pub/Sub and core NATS.
	Id() string

	// Topic returns the topic the message was published on, which, for a
	// subscription to a NATS wildcard subject, is the subject matching it.
	Topic() string

	// Payload returns the bytes of the message.
	Payload() []byte

	// Headers returns the headers of the message, never nil.
	Headers() map[string]string
}

// BrokerHandler handles a message received by a subscription of an
// [IBroker], with the context of the subscription, done once it ends.
// Returning nil acknowledges the message, while returning an error, or
// panicking, leaves it unacknowledged, to be delivered again where the
// backend supports it, as each broker driver documents. The handlers of a
// subscription are called one at a time, in the order the backend
// delivers the messages.
type BrokerHandler func(ctx context.Context, msg IBrokerMessage) error

// IBrokerSubscription is a subscription of an [IBroker] to a topic,
// calling its handler, from a goroutine of its own, with each message
// published on the topic, until it ends: when closed, when the context
// given to Subscribe is done, when its broker closes, or when receiving
// fails.
type IBrokerSubscription interface {
	io.Closer

	// Topic returns the topic of the subscription.
	Topic() string

	// Done returns a channel closed once the subscription ended, its
	// handler no longer being called.
	Done() <-chan struct{}

	// Err returns the error that ended the subscription, nil while it runs
	// or when it ended by being closed or by its context being done.
	Err() error
}

// IBroker publishes the messages of an application to a distributed
// messaging backend, such as Redis, NATS or Kafka, and subscribes to them.
// Unlike an [IPubSub], which is in-process and typed, a broker moves
// opaque bytes between processes: [github.com/cjdias/flam-in-go/pkg/flam/broker.PublishJSON]
// and [github.com/cjdias/flam-in-go/pkg/flam/broker.SubscribeJSON] carry
// typed values as JSON.
//
// A broker is held by an [IBrokerFactory], which closes it when removing
// it. Closing it closes its subscriptions, after which Publish and
// Subscribe fail with
// [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrClosed]. Whether a
// message is delivered at most or at least once, and to every subscriber
// or to one of a group, depends on the driver and its configuration, as
// each driver documents. A broker is safe for concurrent use.
type IBroker interface {
	io.Closer
	IResource

	// Publish sends the payload, with the headers, which may be nil, on
	// the topic, returning once the backend, or the client buffering for
	// it, accepted it, as the driver documents, or failing with the error
	// wrapping [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrPublish].
	// It fails with the error wrapping
	// [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrInvalidTopic] when
	// the topic is empty.
	Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) error

	// Subscribe subscribes the handler to the messages published on the
	// topic, returning once the subscription is in place, as far as the
	// backend tells, or failing with the error wrapping
	// [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrSubscribe], or
	// [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrInvalidTopic] or
	// [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrNilHandler]. The
	// subscription ends when the context is done, or when closed.
	Subscribe(ctx context.Context, topic string, handler BrokerHandler) (IBrokerSubscription, error)
}

// IBrokerCreator creates the brokers of a driver, for the
// [IBrokerFactory].
type IBrokerCreator = IResourceCreator[IBroker]

// IBrokerFactory creates and holds the brokers, one per id.
type IBrokerFactory = IFactory[IBroker]

// IBrokerKafkaWriter is the part of a [kafka.Writer] of
// [github.com/segmentio/kafka-go] the Kafka broker driver publishes with,
// so that tests can fake it.
type IBrokerKafkaWriter interface {
	io.Closer

	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

// IBrokerKafkaReader is the part of a [kafka.Reader] of
// [github.com/segmentio/kafka-go] the Kafka broker driver subscribes with,
// so that tests can fake it.
type IBrokerKafkaReader interface {
	io.Closer

	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

// IBrokerKafkaClient makes the writers and readers of the Kafka broker
// driver, which uses kafka-go itself unless the application container
// provides one, such as a fake reaching no Kafka cluster in tests.
type IBrokerKafkaClient interface {
	// Writer returns the writer publishing with the writer configured by
	// the driver, which the real client returns as is.
	Writer(writer *kafka.Writer) IBrokerKafkaWriter

	// Reader returns the reader of the configuration, made by the real
	// client with [kafka.NewReader].
	Reader(config kafka.ReaderConfig) IBrokerKafkaReader
}
