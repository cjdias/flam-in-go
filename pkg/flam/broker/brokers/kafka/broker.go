package kafka

import (
	"context"
	"errors"
	"strconv"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/internal/base"
)

// kafkaBroker is the [flam.IBroker] of the Kafka driver, owning its
// writer and the readers of its subscriptions.
type kafkaBroker struct {
	id           string
	client       flam.IBrokerKafkaClient
	writer       flam.IBrokerKafkaWriter
	reader       kafkago.ReaderConfig
	retryBackoff time.Duration
	registry     *base.Registry
}

// compile time check that *kafkaBroker implements IBroker
var _ flam.IBroker = (*kafkaBroker)(nil)

func (b *kafkaBroker) Id() string {
	return b.id
}

// Publish writes the message on the topic, its [HeaderKey] header, if
// any, being its key, returning once its batch is written, as the
// acknowledgements of the broker require, or at once when asynchronous.
func (b *kafkaBroker) Publish(
	ctx context.Context,
	topic string,
	payload []byte,
	headers map[string]string,
) error {
	if err := b.registry.CheckPublish(brokerPublishMessage, topic); err != nil {
		return err
	}

	msg := kafkago.Message{Topic: topic, Value: payload}
	for name, value := range headers {
		if name == HeaderKey {
			msg.Key = []byte(value)
			continue
		}
		msg.Headers = append(msg.Headers, kafkago.Header{Key: name, Value: []byte(value)})
	}
	if err := b.writer.WriteMessages(ctx, msg); err != nil {
		return base.NewErrTopic(broker.ErrPublish, brokerPublishMessage, b.id, topic, err)
	}
	return nil
}

// Subscribe reads the topic with a reader of its own in the consumer group
// of the broker, which joins the group in the background, handling each
// message until its handler succeeds, waiting for the retry backoff
// between two attempts, then committing its offset.
func (b *kafkaBroker) Subscribe(
	ctx context.Context,
	topic string,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	if err := b.registry.CheckSubscribe(brokerSubscribeMessage, topic, handler); err != nil {
		return nil, err
	}
	sub := base.NewSubscription(ctx, topic)
	config := b.reader
	config.Topic = topic
	reader := b.client.Reader(config)
	run := func(ctx context.Context) error {
		return b.read(ctx, topic, reader, handler)
	}
	return b.registry.Start(brokerSubscribeMessage, sub, run, reader.Close)
}

// Close closes the subscriptions of the broker, then its writer, returning
// their errors joined.
func (b *kafkaBroker) Close() error {
	return errors.Join(b.registry.Close(), b.writer.Close())
}

// Messages of the errors of the broker.
const (
	brokerPublishMessage   = "unable to publish kafka broker message"
	brokerSubscribeMessage = "unable to subscribe to kafka broker topic"
	brokerReceiveMessage   = "unable to receive kafka broker message"
)

// read fetches the messages of the reader, handling each until its
// handler succeeds, then committing it, until the context is done, or a
// fetch or a commit fails.
func (b *kafkaBroker) read(
	ctx context.Context,
	topic string,
	reader flam.IBrokerKafkaReader,
	handler flam.BrokerHandler,
) error {
	for {
		m, err := reader.FetchMessage(ctx)
		if err != nil {
			return base.ReceiveErr(ctx, brokerReceiveMessage, b.id, topic, err)
		}
		msg := message(m)
		for base.Handle(ctx, b.id, handler, msg) != nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(b.retryBackoff):
			}
		}
		if err := reader.CommitMessages(ctx, m); err != nil {
			return base.ReceiveErr(ctx, brokerReceiveMessage, b.id, topic, err)
		}
	}
}

// message returns the message of the Kafka message, its id being its
// partition and offset, such as "0:42", and its key, if any, its
// [HeaderKey] header.
func message(
	m kafkago.Message,
) flam.IBrokerMessage {
	headers := make(map[string]string, len(m.Headers)+1)
	for _, header := range m.Headers {
		headers[header.Key] = string(header.Value)
	}
	if len(m.Key) > 0 {
		headers[HeaderKey] = string(m.Key)
	}
	id := strconv.Itoa(m.Partition) + ":" + strconv.FormatInt(m.Offset, 10)
	return broker.NewMessage(id, m.Topic, m.Value, headers)
}
