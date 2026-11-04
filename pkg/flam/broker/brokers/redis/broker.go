package redis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/internal/base"
)

// redisBroker is the [flam.IBroker] of the redis driver, publishing and
// subscribing over a redis connection it does not own.
type redisBroker struct {
	id        string
	conn      flam.IRedisConnection
	mode      string
	group     string
	consumer  string
	start     string
	batch     int64
	block     time.Duration
	claimIdle time.Duration
	maxLen    int64
	registry  *base.Registry
	telemetry *base.Telemetry
}

// compile time checks that *redisBroker implements IBroker and
// IHealthPinger
var (
	_ flam.IBroker       = (*redisBroker)(nil)
	_ flam.IHealthPinger = (*redisBroker)(nil)
)

func (b *redisBroker) Id() string {
	return b.id
}

// Publish publishes the message with PUBLISH, as a JSON envelope holding
// the payload and the headers, in the pubsub mode, or adds it to the
// stream of the topic with XADD, as the [StreamFieldPayload] field and a
// field per header, in the streams mode, traced and measured by the
// telemetry of the broker.
func (b *redisBroker) Publish(
	ctx context.Context,
	topic string,
	payload []byte,
	headers map[string]string,
) error {
	if err := b.registry.CheckPublish(brokerPublishMessage, topic); err != nil {
		return err
	}
	return b.telemetry.Publish(ctx, topic, headers, func(ctx context.Context, headers map[string]string) error {
		return b.publish(ctx, topic, payload, headers)
	})
}

// Subscribe subscribes to the channel of the topic with SUBSCRIBE in the
// pubsub mode, or reads the stream of the topic with XREADGROUP in the
// consumer group of the broker in the streams mode, creating the group,
// and the stream, when missing.
func (b *redisBroker) Subscribe(
	ctx context.Context,
	topic string,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	if err := b.registry.CheckSubscribe(brokerSubscribeMessage, topic, handler); err != nil {
		return nil, err
	}
	sub := base.NewSubscription(ctx, topic)
	if b.mode == ModePubSub {
		return b.subscribeChannel(sub, handler)
	}
	return b.subscribeStream(sub, handler)
}

// Ping sends PING over the connection of the broker, for the health
// checks, a nil context being the background one.
func (b *redisBroker) Ping(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return b.conn.Ping(ctx).Err()
}

// Close closes the subscriptions of the broker, but not its connection,
// owned by the connection factory.
func (b *redisBroker) Close() error {
	return b.registry.Close()
}

// Messages of the errors of the broker.
const (
	brokerPublishMessage   = "unable to publish redis broker message"
	brokerSubscribeMessage = "unable to subscribe to redis broker topic"
)

// publish publishes the message in the mode of the broker.
func (b *redisBroker) publish(
	ctx context.Context,
	topic string,
	payload []byte,
	headers map[string]string,
) error {
	var err error
	if b.mode == ModePubSub {
		data, _ := json.Marshal(envelope{Headers: headers, Payload: payload}) // a map of strings and bytes always encode
		err = b.conn.Publish(ctx, topic, data).Err()
	} else {
		values := []any{StreamFieldPayload, payload}
		for name, value := range headers {
			values = append(values, StreamFieldHeaderPrefix+name, value)
		}
		err = b.conn.XAdd(ctx, &goredis.XAddArgs{
			Stream: topic,
			MaxLen: b.maxLen,
			Approx: b.maxLen > 0,
			Values: values,
		}).Err()
	}
	if err != nil {
		return base.NewErrTopic(broker.ErrPublish, brokerPublishMessage, b.id, topic, err)
	}
	return nil
}

// envelope is the JSON encoding of a message of the pubsub mode, its
// payload being encoded in base64, or as null when nil.
type envelope struct {
	Headers map[string]string `json:"headers,omitempty"`
	Payload []byte            `json:"payload"`
}

// rawEnvelope is the JSON decoding of an envelope, its payload being nil
// when the "payload" key is missing, and "null" when its value is.
type rawEnvelope struct {
	Headers map[string]string `json:"headers"`
	Payload json.RawMessage   `json:"payload"`
}

// errChannelClosed is the cause of the error ending a subscription of the
// pubsub mode when go-redis closes its channel, as when its client is
// closed.
var errChannelClosed = errors.New("redis broker channel closed")

// subscribeChannel subscribes to the channel of the topic, waiting for
// the confirmation of the server, then hands the messages to the handler
// until the subscription ends.
func (b *redisBroker) subscribeChannel(
	sub *base.Subscription,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	ctx := sub.Context()
	ps := b.conn.Subscribe(ctx, sub.Topic())
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return nil, base.NewErrTopic(broker.ErrSubscribe, brokerSubscribeMessage, b.id, sub.Topic(), err)
	}
	run := func(ctx context.Context) error {
		messages := ps.Channel()
		for {
			select {
			case <-ctx.Done():
				return nil
			case m, ok := <-messages:
				if !ok {
					return base.ReceiveErr(ctx, brokerReceiveMessage, b.id, sub.Topic(), errChannelClosed)
				}
				_ = b.telemetry.Handle(ctx, sub.Topic(), handler, channelMessage(m))
			}
		}
	}
	return b.registry.Start(brokerSubscribeMessage, sub, run, ps.Close)
}

// channelMessage returns the message of the pubsub mode, decoded from its
// envelope, a JSON object holding the "payload" key, whose null value is
// an empty payload, or holding the whole data as its payload when not an
// envelope, as when published by another client.
func channelMessage(
	m *goredis.Message,
) flam.IBrokerMessage {
	var e rawEnvelope
	var payload []byte
	if json.Unmarshal([]byte(m.Payload), &e) != nil || e.Payload == nil || json.Unmarshal(e.Payload, &payload) != nil {
		return broker.NewMessage("", m.Channel, []byte(m.Payload), nil)
	}
	return broker.NewMessage("", m.Channel, payload, e.Headers)
}

// subscribeStream creates the consumer group of the stream of the topic,
// unless it exists, then reads the stream in the group until the
// subscription ends.
func (b *redisBroker) subscribeStream(
	sub *base.Subscription,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	start := "$"
	if b.start == StartAll {
		start = "0"
	}
	err := b.conn.XGroupCreateMkStream(sub.Context(), sub.Topic(), b.group, start).Err()
	if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		return nil, base.NewErrTopic(broker.ErrSubscribe, brokerSubscribeMessage, b.id, sub.Topic(), err)
	}
	consumer := b.consumer
	if consumer == "" {
		consumer = consumerName(b.id)
	}
	run := func(ctx context.Context) error {
		return b.readStream(ctx, sub.Topic(), consumer, handler)
	}
	release := func() error {
		if b.consumer == "" {
			b.removeConsumer(context.WithoutCancel(sub.Context()), sub.Topic(), consumer)
		}
		return nil
	}
	return b.registry.Start(brokerSubscribeMessage, sub, run, release)
}

// removeConsumer deletes the consumer from the group of the stream of the
// topic with XGROUP DELCONSUMER, unless it holds pending entries, which
// deleting it would drop, leaving it for them to be claimed by the other
// consumers of the group. It does its best, a failure leaving the
// consumer in place.
func (b *redisBroker) removeConsumer(
	ctx context.Context,
	topic string,
	consumer string,
) {
	pending, err := b.conn.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream:   topic,
		Group:    b.group,
		Start:    "-",
		End:      "+",
		Count:    1,
		Consumer: consumer,
	}).Result()
	if err == nil && len(pending) == 0 {
		_ = b.conn.XGroupDelConsumer(ctx, topic, b.group, consumer).Err() // left in place, it is harmless
	}
}

// readStream reads the stream of the topic as the consumer of the group,
// first claiming the entries left unacknowledged for the claim idle time,
// until the context is done, or a read fails.
func (b *redisBroker) readStream(
	ctx context.Context,
	topic string,
	consumer string,
	handler flam.BrokerHandler,
) error {
	for ctx.Err() == nil {
		if b.claimIdle > 0 {
			entries, _, err := b.conn.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
				Stream:   topic,
				Group:    b.group,
				Consumer: consumer,
				MinIdle:  b.claimIdle,
				Start:    "0-0",
				Count:    b.batch,
			}).Result()
			if err != nil {
				return base.ReceiveErr(ctx, brokerReceiveMessage, b.id, topic, err)
			}
			b.handleEntries(ctx, topic, entries, handler)
		}

		streams, err := b.conn.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    b.group,
			Consumer: consumer,
			Streams:  []string{topic, ">"},
			Count:    b.batch,
			Block:    b.block,
		}).Result()
		switch {
		case errors.Is(err, goredis.Nil):
			continue
		case err != nil:
			return base.ReceiveErr(ctx, brokerReceiveMessage, b.id, topic, err)
		}
		for _, stream := range streams {
			b.handleEntries(ctx, topic, stream.Messages, handler)
		}
	}
	return nil
}

// handleEntries hands the entries to the handler, acknowledging those it
// handled, until the context is done, the others staying pending, to be
// claimed again.
func (b *redisBroker) handleEntries(
	ctx context.Context,
	topic string,
	entries []goredis.XMessage,
	handler flam.BrokerHandler,
) {
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		if b.telemetry.Handle(ctx, topic, handler, streamMessage(topic, entry)) == nil {
			_ = b.conn.XAck(ctx, topic, b.group, entry.ID).Err() // unacknowledged, it is claimed again
		}
	}
}

// streamMessage returns the message of the entry of the stream of the
// topic, its payload being the [StreamFieldPayload] field and its headers
// the fields prefixed by [StreamFieldHeaderPrefix].
func streamMessage(
	topic string,
	entry goredis.XMessage,
) flam.IBrokerMessage {
	payload, _ := entry.Values[StreamFieldPayload].(string)
	headers := map[string]string{}
	for field, value := range entry.Values {
		if name, ok := strings.CutPrefix(field, StreamFieldHeaderPrefix); ok {
			headers[name], _ = value.(string)
		}
	}
	return broker.NewMessage(entry.ID, topic, []byte(payload), headers)
}

// brokerReceiveMessage is the message of the errors ending a subscription.
const brokerReceiveMessage = "unable to receive redis broker message"
