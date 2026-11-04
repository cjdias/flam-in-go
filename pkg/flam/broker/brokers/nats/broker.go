package nats

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/internal/base"
)

// natsBroker is the [flam.IBroker] of the NATS driver, owning its
// connection.
type natsBroker struct {
	id         string
	conn       *natsgo.Conn
	js         jetstream.JetStream
	queue      string
	jetStream  bool
	stream     string
	durable    string
	ackWait    time.Duration
	maxDeliver int
	registry   *base.Registry
	telemetry  *base.Telemetry
}

// compile time checks that *natsBroker implements IBroker and
// IHealthPinger
var (
	_ flam.IBroker       = (*natsBroker)(nil)
	_ flam.IHealthPinger = (*natsBroker)(nil)
)

func (b *natsBroker) Id() string {
	return b.id
}

// Publish publishes the message on the subject of the topic, returning
// once the client buffered it with core NATS, or once the stream stored
// it with JetStream, traced and measured by the telemetry of the broker.
func (b *natsBroker) Publish(
	ctx context.Context,
	topic string,
	payload []byte,
	headers map[string]string,
) error {
	if err := b.registry.CheckPublish(brokerPublishMessage, topic); err != nil {
		return err
	}
	return b.telemetry.Publish(ctx, topic, headers, func(ctx context.Context, headers map[string]string) error {
		msg := &natsgo.Msg{Subject: topic, Data: payload, Header: natsgo.Header{}}
		for name, value := range headers {
			msg.Header.Set(name, value)
		}
		var err error
		if b.jetStream {
			_, err = b.js.PublishMsg(ctx, msg)
		} else {
			err = b.conn.PublishMsg(msg)
		}
		if err != nil {
			return base.NewErrTopic(broker.ErrPublish, brokerPublishMessage, b.id, topic, err)
		}
		return nil
	})
}

// Subscribe subscribes to the subject of the topic, which may hold
// wildcards, in the queue group of the broker, if any, with core NATS, or
// creates or updates the durable consumer of the topic on the stream and
// consumes it with JetStream.
func (b *natsBroker) Subscribe(
	ctx context.Context,
	topic string,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	if err := b.registry.CheckSubscribe(brokerSubscribeMessage, topic, handler); err != nil {
		return nil, err
	}
	sub := base.NewSubscription(ctx, topic)
	if b.jetStream {
		return b.consume(sub, handler)
	}
	return b.subscribe(sub, handler)
}

// Ping flushes the connection of the broker, waiting for the server to
// answer, for the health checks, within pingTimeout when the context has
// no deadline, a nil context being the background one.
func (b *natsBroker) Ping(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, pingTimeout)
		defer cancel()
	}
	return b.conn.FlushWithContext(ctx)
}

// pingTimeout is the time a Ping given a context without deadline waits
// for the server.
const pingTimeout = 5 * time.Second

// Close closes the subscriptions of the broker, then its connection.
func (b *natsBroker) Close() error {
	err := b.registry.Close()
	b.conn.Close()
	return err
}

// Messages of the errors of the broker.
const (
	brokerPublishMessage   = "unable to publish nats broker message"
	brokerSubscribeMessage = "unable to subscribe to nats broker topic"
)

// subscribe subscribes to the subject of the topic with core NATS,
// flushing the connection so that the server holds the subscription once
// it returns. Its release unsubscribes, then waits for the handler under
// way, as unsubscribing does not.
func (b *natsBroker) subscribe(
	sub *base.Subscription,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	ctx := sub.Context()
	var handling sync.Mutex // held while the handler runs, for the release to wait for it
	callback := func(m *natsgo.Msg) {
		handling.Lock()
		defer handling.Unlock()
		if ctx.Err() == nil {
			_ = b.telemetry.Handle(ctx, sub.Topic(), handler, message("", m.Subject, m.Data, m.Header))
		}
	}

	var ns *natsgo.Subscription
	var err error
	if b.queue == "" {
		ns, err = b.conn.Subscribe(sub.Topic(), callback)
	} else {
		ns, err = b.conn.QueueSubscribe(sub.Topic(), b.queue, callback)
	}
	if err == nil {
		err = b.conn.Flush()
	}
	if err != nil {
		_ = unsubscribe(ns)
		return nil, base.NewErrTopic(broker.ErrSubscribe, brokerSubscribeMessage, b.id, sub.Topic(), err)
	}
	return b.start(sub, func() error {
		err := unsubscribe(ns)
		handling.Lock() // the context being done, no handler runs once locked
		defer handling.Unlock()
		return err
	})
}

// unsubscribe removes the subscription, when not nil.
func unsubscribe(
	ns *natsgo.Subscription,
) error {
	if ns == nil {
		return nil
	}
	return ns.Unsubscribe()
}

// consume creates or updates the durable consumer of the topic on the
// stream, acknowledging explicitly, delivering the messages published
// from then on, and consumes it, acknowledging the messages handled, and
// asking for the others to be delivered again.
func (b *natsBroker) consume(
	sub *base.Subscription,
	handler flam.BrokerHandler,
) (flam.IBrokerSubscription, error) {
	ctx := sub.Context()
	callback := func(m jetstream.Msg) {
		id := ""
		if metadata, err := m.Metadata(); err == nil {
			id = strconv.FormatUint(metadata.Sequence.Stream, 10)
		}
		if ctx.Err() == nil && b.telemetry.Handle(ctx, sub.Topic(), handler, message(id, m.Subject(), m.Data(), m.Headers())) == nil {
			_ = m.Ack() // unacknowledged, it is delivered again after the ack wait
			return
		}
		_ = m.Nak()
	}

	var consumeCtx jetstream.ConsumeContext
	consumer, err := b.js.CreateOrUpdateConsumer(ctx, b.stream, jetstream.ConsumerConfig{
		Durable:       durableName(b.durable, sub.Topic()),
		FilterSubject: sub.Topic(),
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       b.ackWait,
		MaxDeliver:    b.maxDeliver,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err == nil {
		consumeCtx, err = consumer.Consume(callback)
	}
	if err != nil {
		return nil, base.NewErrTopic(broker.ErrSubscribe, brokerSubscribeMessage, b.id, sub.Topic(), err)
	}
	return b.start(sub, func() error {
		consumeCtx.Stop()
		<-consumeCtx.Closed()
		return nil
	})
}

// start runs the subscription until its context is done, then releases
// it.
func (b *natsBroker) start(
	sub *base.Subscription,
	release func() error,
) (flam.IBrokerSubscription, error) {
	run := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}
	return b.registry.Start(brokerSubscribeMessage, sub, run, release)
}

// message returns the message of the NATS message, taking the first value
// of each header.
func message(
	id string,
	subject string,
	data []byte,
	header natsgo.Header,
) flam.IBrokerMessage {
	headers := make(map[string]string, len(header))
	for name := range header {
		headers[name] = header.Get(name)
	}
	return broker.NewMessage(id, subject, data, headers)
}

// durableName returns the name of the durable consumer of the topic,
// made of the prefix and the topic, the characters a consumer name cannot
// hold replaced by underscores, cut to durableNameMax bytes, followed by
// the start of the hex SHA-256 of the exact prefix and topic, so that the
// topics differing only by those characters, or by what was cut, such as
// "a.b" and "a_b", or "orders.*" and "orders.>", have consumers of their
// own.
func durableName(
	prefix string,
	topic string,
) string {
	sum := sha256.Sum256([]byte(prefix + "\x00" + topic))
	name := durableReplacer.Replace(prefix + "_" + topic)
	if len(name) > durableNameMax {
		name = strings.ToValidUTF8(name[:durableNameMax], "") // drops a rune cut in two
	}
	return name + "_" + hex.EncodeToString(sum[:durableHashBytes])
}

// durableReplacer replaces by underscores the characters a consumer name
// cannot hold.
var durableReplacer = strings.NewReplacer(
	".", "_", "*", "_", ">", "_", " ", "_", "/", "_", "\\", "_",
	"\t", "_", "\r", "_", "\n", "_", "\f", "_",
)

// Bounds of the names of the durable consumers, well within the 255 bytes
// allowed by JetStream.
const (
	// durableNameMax is the maximum length of the readable part of a name.
	durableNameMax = 64

	// durableHashBytes is the number of bytes of the hash ending a name,
	// written in twice as many hex digits.
	durableHashBytes = 8
)
