package brokers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	brokerkafka "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/kafka"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// fakeWriter is a flam.IBrokerKafkaWriter recording the messages written.
type fakeWriter struct {
	err      error
	closeErr error

	mu       sync.Mutex
	messages []kafkago.Message
	closed   bool
}

func (w *fakeWriter) WriteMessages(_ context.Context, msgs ...kafkago.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, msgs...)
	return w.err
}

func (w *fakeWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return w.closeErr
}

// fakeReader is a flam.IBrokerKafkaReader fetching the messages and the
// errors sent to its channels, and recording the commits.
type fakeReader struct {
	config    kafkago.ReaderConfig
	messages  chan kafkago.Message
	errs      chan error
	commitErr error
	closeErr  error

	mu      sync.Mutex
	commits []kafkago.Message
	closed  bool
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	select {
	case m := <-r.messages:
		return m, nil
	case err := <-r.errs:
		return kafkago.Message{}, err
	case <-ctx.Done():
		return kafkago.Message{}, ctx.Err()
	}
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commits = append(r.commits, msgs...)
	return r.commitErr
}

func (r *fakeReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return r.closeErr
}

func (r *fakeReader) committed() []kafkago.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]kafkago.Message(nil), r.commits...)
}

func (r *fakeReader) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// fakeKafka is a flam.IBrokerKafkaClient of a fake writer and of fake
// readers, calling onReader, when set, with each reader made.
type fakeKafka struct {
	writer    *fakeWriter
	commitErr error
	closeErr  error
	onReader  func(r *fakeReader)

	mu      sync.Mutex
	written *kafkago.Writer
	readers []*fakeReader
}

func newFakeKafka() *fakeKafka {
	return &fakeKafka{writer: &fakeWriter{}}
}

func (k *fakeKafka) Writer(w *kafkago.Writer) flam.IBrokerKafkaWriter {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.written = w
	return k.writer
}

func (k *fakeKafka) Reader(config kafkago.ReaderConfig) flam.IBrokerKafkaReader {
	r := &fakeReader{
		config:    config,
		messages:  make(chan kafkago.Message, 10),
		errs:      make(chan error, 1),
		commitErr: k.commitErr,
		closeErr:  k.closeErr,
	}
	k.mu.Lock()
	k.readers = append(k.readers, r)
	k.mu.Unlock()
	if k.onReader != nil {
		k.onReader(r)
	}
	return r
}

func (k *fakeKafka) reader(i int) *fakeReader {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.readers[i]
}

// kafkaBroker returns the Kafka broker "main" of the fields, using the
// fake client.
func kafkaBroker(t *testing.T, k *fakeKafka, fields container.Bag) flam.IBroker {
	t.Helper()
	return mustCreate(t, brokerkafka.Driver, fields, nil, func(a *testutil.App) {
		testutil.NoError(t, a.Provide(func() flam.IBrokerKafkaClient { return k }))
	})
}

func Test_Kafka_Declarations(t *testing.T) {
	b := brokerkafka.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.broker.brokers.kafka")
	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		brokerkafka.DefaultBrokersConfigPath:      []any{"127.0.0.1:9092"},
		brokerkafka.DefaultBalancerConfigPath:     "hash",
		brokerkafka.DefaultRequiredAcksConfigPath: "all",
		brokerkafka.DefaultBatchSizeConfigPath:    100,
		brokerkafka.DefaultBatchBytesConfigPath:   1048576,
		brokerkafka.DefaultBatchTimeoutConfigPath: "10ms",
		brokerkafka.DefaultWriteTimeoutConfigPath: "10s",
		brokerkafka.DefaultMaxAttemptsConfigPath:  10,
		brokerkafka.DefaultMinBytesConfigPath:     1,
		brokerkafka.DefaultMaxBytesConfigPath:     10485760,
		brokerkafka.DefaultMaxWaitConfigPath:      "1s",
		brokerkafka.DefaultStartOffsetConfigPath:  "last",
		brokerkafka.DefaultRetryBackoffConfigPath: "1s",
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}

	for _, pair := range [][2]string{
		{brokerkafka.Driver, "flam.broker.brokers.driver.kafka"},
		{brokerkafka.HeaderKey, "kafka.key"},
		{brokerkafka.BalancerHash, "hash"},
		{brokerkafka.BalancerRoundRobin, "round_robin"},
		{brokerkafka.BalancerLeastBytes, "least_bytes"},
		{brokerkafka.AcksNone, "none"},
		{brokerkafka.AcksOne, "one"},
		{brokerkafka.AcksAll, "all"},
		{brokerkafka.OffsetFirst, "first"},
		{brokerkafka.OffsetLast, "last"},
		{brokerkafka.ConfigFieldBrokers, "brokers"},
		{brokerkafka.ConfigFieldGroupID, "group_id"},
		{brokerkafka.ConfigFieldBalancer, "balancer"},
		{brokerkafka.ConfigFieldRequiredAcks, "required_acks"},
		{brokerkafka.ConfigFieldAsync, "async"},
		{brokerkafka.ConfigFieldBatchSize, "batch_size"},
		{brokerkafka.ConfigFieldBatchBytes, "batch_bytes"},
		{brokerkafka.ConfigFieldBatchTimeout, "batch_timeout"},
		{brokerkafka.ConfigFieldWriteTimeout, "write_timeout"},
		{brokerkafka.ConfigFieldMaxAttempts, "max_attempts"},
		{brokerkafka.ConfigFieldAutoCreateTopic, "auto_create_topic"},
		{brokerkafka.ConfigFieldMinBytes, "min_bytes"},
		{brokerkafka.ConfigFieldMaxBytes, "max_bytes"},
		{brokerkafka.ConfigFieldMaxWait, "max_wait"},
		{brokerkafka.ConfigFieldStartOffset, "start_offset"},
		{brokerkafka.ConfigFieldCommitInterval, "commit_interval"},
		{brokerkafka.ConfigFieldRetryBackoff, "retry_backoff"},
		{brokerkafka.DefaultBrokersConfigPath, "flam.broker.defaults.brokers.kafka.brokers"},
		{brokerkafka.DefaultBalancerConfigPath, "flam.broker.defaults.brokers.kafka.balancer"},
		{brokerkafka.DefaultRequiredAcksConfigPath, "flam.broker.defaults.brokers.kafka.required_acks"},
		{brokerkafka.DefaultBatchSizeConfigPath, "flam.broker.defaults.brokers.kafka.batch_size"},
		{brokerkafka.DefaultBatchBytesConfigPath, "flam.broker.defaults.brokers.kafka.batch_bytes"},
		{brokerkafka.DefaultBatchTimeoutConfigPath, "flam.broker.defaults.brokers.kafka.batch_timeout"},
		{brokerkafka.DefaultWriteTimeoutConfigPath, "flam.broker.defaults.brokers.kafka.write_timeout"},
		{brokerkafka.DefaultMaxAttemptsConfigPath, "flam.broker.defaults.brokers.kafka.max_attempts"},
		{brokerkafka.DefaultMinBytesConfigPath, "flam.broker.defaults.brokers.kafka.min_bytes"},
		{brokerkafka.DefaultMaxBytesConfigPath, "flam.broker.defaults.brokers.kafka.max_bytes"},
		{brokerkafka.DefaultMaxWaitConfigPath, "flam.broker.defaults.brokers.kafka.max_wait"},
		{brokerkafka.DefaultStartOffsetConfigPath, "flam.broker.defaults.brokers.kafka.start_offset"},
		{brokerkafka.DefaultRetryBackoffConfigPath, "flam.broker.defaults.brokers.kafka.retry_backoff"},
		{brokerkafka.ErrorFieldID, "id"},
		{brokerkafka.ErrorFieldField, "field"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("constant = %q, want %q", pair[0], pair[1])
		}
	}
}

func Test_Kafka_Create(t *testing.T) {
	ctx := context.Background()
	noop := func(context.Context, flam.IBrokerMessage) error { return nil }

	t.Run("should configure the writer and the readers with the defaults", func(t *testing.T) {
		k := newFakeKafka()
		b := kafkaBroker(t, k, container.Bag{})
		w := k.written
		testutil.Equal(t, "Addr", w.Addr.String(), "127.0.0.1:9092")
		testutil.Equal(t, "Balancer", w.Balancer, kafkago.Balancer(&kafkago.Hash{}))
		testutil.Equal(t, "RequiredAcks", w.RequiredAcks, kafkago.RequireAll)
		testutil.Equal(t, "Async", w.Async, false)
		testutil.Equal(t, "BatchSize", w.BatchSize, 100)
		testutil.Equal(t, "BatchBytes", w.BatchBytes, int64(1048576))
		testutil.Equal(t, "BatchTimeout", w.BatchTimeout, 10*time.Millisecond)
		testutil.Equal(t, "WriteTimeout", w.WriteTimeout, 10*time.Second)
		testutil.Equal(t, "MaxAttempts", w.MaxAttempts, 10)
		testutil.Equal(t, "AllowAutoTopicCreation", w.AllowAutoTopicCreation, false)

		_, err := b.Subscribe(ctx, "orders", noop)
		testutil.NoError(t, err)
		testutil.Equal(t, "reader", k.reader(0).config, kafkago.ReaderConfig{
			Brokers:     []string{"127.0.0.1:9092"},
			GroupID:     "main",
			Topic:       "orders",
			MinBytes:    1,
			MaxBytes:    10485760,
			MaxWait:     time.Second,
			StartOffset: kafkago.LastOffset,
		})
	})

	t.Run("should configure the writer and the readers with the fields", func(t *testing.T) {
		k := newFakeKafka()
		b := kafkaBroker(t, k, container.Bag{
			brokerkafka.ConfigFieldBrokers:         []any{"a:1", "b:2"},
			brokerkafka.ConfigFieldGroupID:         "group",
			brokerkafka.ConfigFieldBalancer:        brokerkafka.BalancerRoundRobin,
			brokerkafka.ConfigFieldRequiredAcks:    brokerkafka.AcksOne,
			brokerkafka.ConfigFieldAsync:           true,
			brokerkafka.ConfigFieldBatchSize:       5,
			brokerkafka.ConfigFieldBatchBytes:      6,
			brokerkafka.ConfigFieldBatchTimeout:    "7ms",
			brokerkafka.ConfigFieldWriteTimeout:    "8s",
			brokerkafka.ConfigFieldMaxAttempts:     9,
			brokerkafka.ConfigFieldAutoCreateTopic: true,
			brokerkafka.ConfigFieldMinBytes:        10,
			brokerkafka.ConfigFieldMaxBytes:        10,
			brokerkafka.ConfigFieldMaxWait:         "11ms",
			brokerkafka.ConfigFieldStartOffset:     brokerkafka.OffsetFirst,
			brokerkafka.ConfigFieldCommitInterval:  "12ms",
		})
		w := k.written
		testutil.Equal(t, "Addr", w.Addr.String(), "a:1,b:2")
		testutil.Equal(t, "Balancer", w.Balancer, kafkago.Balancer(&kafkago.RoundRobin{}))
		testutil.Equal(t, "RequiredAcks", w.RequiredAcks, kafkago.RequireOne)
		testutil.Equal(t, "Async", w.Async, true)
		testutil.Equal(t, "BatchSize", w.BatchSize, 5)
		testutil.Equal(t, "BatchBytes", w.BatchBytes, int64(6))
		testutil.Equal(t, "BatchTimeout", w.BatchTimeout, 7*time.Millisecond)
		testutil.Equal(t, "WriteTimeout", w.WriteTimeout, 8*time.Second)
		testutil.Equal(t, "MaxAttempts", w.MaxAttempts, 9)
		testutil.Equal(t, "AllowAutoTopicCreation", w.AllowAutoTopicCreation, true)

		_, err := b.Subscribe(ctx, "orders", noop)
		testutil.NoError(t, err)
		testutil.Equal(t, "reader", k.reader(0).config, kafkago.ReaderConfig{
			Brokers:        []string{"a:1", "b:2"},
			GroupID:        "group",
			Topic:          "orders",
			MinBytes:       10,
			MaxBytes:       10,
			MaxWait:        11 * time.Millisecond,
			StartOffset:    kafkago.FirstOffset,
			CommitInterval: 12 * time.Millisecond,
		})

		other := newFakeKafka()
		kafkaBroker(t, other, container.Bag{
			brokerkafka.ConfigFieldBalancer:     brokerkafka.BalancerLeastBytes,
			brokerkafka.ConfigFieldRequiredAcks: brokerkafka.AcksNone,
		})
		testutil.Equal(t, "Balancer", other.written.Balancer, kafkago.Balancer(&kafkago.LeastBytes{}))
		testutil.Equal(t, "RequiredAcks", other.written.RequiredAcks, kafkago.RequireNone)
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, value := range map[string]any{
			brokerkafka.ConfigFieldBrokers:         []any{},
			brokerkafka.ConfigFieldGroupID:         "",
			brokerkafka.ConfigFieldBalancer:        "other",
			brokerkafka.ConfigFieldRequiredAcks:    "other",
			brokerkafka.ConfigFieldAsync:           "yes",
			brokerkafka.ConfigFieldBatchSize:       0,
			brokerkafka.ConfigFieldBatchBytes:      0,
			brokerkafka.ConfigFieldBatchTimeout:    "-1s",
			brokerkafka.ConfigFieldWriteTimeout:    "soon",
			brokerkafka.ConfigFieldMaxAttempts:     0,
			brokerkafka.ConfigFieldAutoCreateTopic: 1,
			brokerkafka.ConfigFieldMinBytes:        0,
			brokerkafka.ConfigFieldMaxBytes:        0,
			brokerkafka.ConfigFieldMaxWait:         1,
			brokerkafka.ConfigFieldStartOffset:     "middle",
			brokerkafka.ConfigFieldCommitInterval:  "-1s",
			brokerkafka.ConfigFieldRetryBackoff:    "-1s",
		} {
			_, err := create(t, brokerkafka.Driver, container.Bag{field: value}, nil)
			assertInvalid(t, err, brokerkafka.ErrInvalidField, field)
		}
		_, err := create(t, brokerkafka.Driver, container.Bag{
			brokerkafka.ConfigFieldMinBytes: 10,
			brokerkafka.ConfigFieldMaxBytes: 9,
		}, nil)
		assertInvalid(t, err, brokerkafka.ErrInvalidField, brokerkafka.ConfigFieldMaxBytes)
		_, err = create(t, brokerkafka.Driver, container.Bag{brokerkafka.ConfigFieldRetryBackoff: "0s"}, nil)
		assertInvalid(t, err, brokerkafka.ErrInvalidField, brokerkafka.ConfigFieldRetryBackoff)
		_, err = create(t, brokerkafka.Driver, container.Bag{}, map[string]any{brokerkafka.DefaultRetryBackoffConfigPath: "0s"})
		assertInvalid(t, err, brokerkafka.ErrInvalidField, brokerkafka.ConfigFieldRetryBackoff)
	})
}

func Test_Kafka_Common(t *testing.T) {
	assertCommon(t, kafkaBroker(t, newFakeKafka(), container.Bag{}), "kafka")
}

func Test_Kafka_Publish(t *testing.T) {
	ctx := context.Background()

	t.Run("should write the message, its key header being its key", func(t *testing.T) {
		k := newFakeKafka()
		b := kafkaBroker(t, k, container.Bag{})
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("one"), map[string]string{"h": "v", brokerkafka.HeaderKey: "k"}))
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("two"), nil))
		testutil.Equal(t, "messages", k.writer.messages, []kafkago.Message{
			{Topic: "orders", Key: []byte("k"), Value: []byte("one"), Headers: []kafkago.Header{{Key: "h", Value: []byte("v")}}},
			{Topic: "orders", Value: []byte("two")},
		})
	})

	t.Run("should fail when the writer fails", func(t *testing.T) {
		k := newFakeKafka()
		k.writer.err = errors.New("boom")
		b := kafkaBroker(t, k, container.Bag{})
		testutil.Error(t, b.Publish(ctx, "orders", nil, nil), broker.ErrPublish,
			"unable to publish kafka broker message: unable to publish broker message: boom",
			map[string]any{"id": "main", "topic": "orders"})
	})

	t.Run("should close the writer with the broker", func(t *testing.T) {
		k := newFakeKafka()
		k.writer.closeErr = errors.New("close")
		b := kafkaBroker(t, k, container.Bag{})
		if err := b.Close(); !errors.Is(err, k.writer.closeErr) {
			t.Fatalf("Close() = %v, want %v", err, k.writer.closeErr)
		}
		testutil.Equal(t, "closed", k.writer.closed, true)
	})
}

func Test_Kafka_Subscribe(t *testing.T) {
	ctx := context.Background()

	t.Run("should handle and commit each message", func(t *testing.T) {
		k := newFakeKafka()
		b := kafkaBroker(t, k, container.Bag{})
		handler, ch := collect(nil)
		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		r := k.reader(0)
		first := kafkago.Message{Topic: "orders", Partition: 1, Offset: 5, Key: []byte("k"), Value: []byte("one"),
			Headers: []kafkago.Header{{Key: "h", Value: []byte("v")}}}
		second := kafkago.Message{Topic: "orders", Partition: 0, Offset: 6, Value: []byte("two")}
		r.messages <- first
		r.messages <- second
		testutil.Equal(t, "first", next(t, ch), received{id: "1:5", topic: "orders", payload: "one",
			headers: map[string]string{"h": "v", brokerkafka.HeaderKey: "k"}})
		testutil.Equal(t, "second", next(t, ch), received{id: "0:6", topic: "orders", payload: "two", headers: map[string]string{}})

		testutil.NoError(t, sub.Close())
		testutil.Equal(t, "commits", r.committed(), []kafkago.Message{first, second})
		testutil.Equal(t, "closed", r.isClosed(), true)
	})

	t.Run("should handle a failed message again until it succeeds", func(t *testing.T) {
		k := newFakeKafka()
		b := kafkaBroker(t, k, container.Bag{brokerkafka.ConfigFieldRetryBackoff: "1ms"})
		var mu sync.Mutex
		attempts := 0
		done := make(chan struct{})
		sub, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error {
			mu.Lock()
			defer mu.Unlock()
			attempts++
			switch attempts {
			case 1:
				return errors.New("failed")
			case 2:
				panic(errors.New("boom"))
			case 3:
				panic("boom")
			}
			close(done)
			return nil
		})
		testutil.NoError(t, err)
		r := k.reader(0)
		r.messages <- kafkago.Message{Topic: "orders", Value: []byte("one")}
		select {
		case <-done:
		case <-time.After(timeout):
			t.Fatal("the message was not handled again")
		}
		testutil.NoError(t, sub.Close())
		testutil.Equal(t, "attempts", attempts, 4)
		testutil.Equal(t, "commits", len(r.committed()), 1)
	})

	t.Run("should stop handling a failed message again once closed", func(t *testing.T) {
		k := newFakeKafka()
		k.closeErr = errors.New("close")
		b := kafkaBroker(t, k, container.Bag{brokerkafka.ConfigFieldRetryBackoff: "1h"})
		handler, ch := collect(func(int) error { return errors.New("failed") })
		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		r := k.reader(0)
		r.messages <- kafkago.Message{Topic: "orders", Value: []byte("one")}
		next(t, ch)
		testutil.Equal(t, "Close()", sub.Close(), k.closeErr)
		testutil.NoError(t, sub.Err())
		testutil.Equal(t, "commits", len(r.committed()), 0)
	})

	t.Run("should end when a fetch or a commit fails", func(t *testing.T) {
		k := newFakeKafka()
		k.commitErr = errors.New("commit")
		b := kafkaBroker(t, k, container.Bag{})
		noop := func(context.Context, flam.IBrokerMessage) error { return nil }

		fetching, err := b.Subscribe(ctx, "orders", noop)
		testutil.NoError(t, err)
		k.reader(0).errs <- errors.New("fetch")
		testutil.Error(t, ended(t, fetching), broker.ErrReceive,
			"unable to receive kafka broker message: unable to receive broker message: fetch",
			map[string]any{"id": "main", "topic": "orders"})

		committing, err := b.Subscribe(ctx, "orders", noop)
		testutil.NoError(t, err)
		k.reader(1).messages <- kafkago.Message{Topic: "orders"}
		testutil.Error(t, ended(t, committing), broker.ErrReceive,
			"unable to receive kafka broker message: unable to receive broker message: commit",
			map[string]any{"id": "main", "topic": "orders"})
	})

	t.Run("should end without error when the commit fails once its context is done", func(t *testing.T) {
		k := newFakeKafka()
		k.commitErr = errors.New("commit")
		b := kafkaBroker(t, k, container.Bag{})
		subCtx, cancel := context.WithCancel(ctx)
		sub, err := b.Subscribe(subCtx, "orders", func(context.Context, flam.IBrokerMessage) error {
			cancel()
			return nil
		})
		testutil.NoError(t, err)
		k.reader(0).messages <- kafkago.Message{Topic: "orders"}
		testutil.NoError(t, ended(t, sub))
	})

	t.Run("should release the reader when the broker closes while subscribing", func(t *testing.T) {
		k := newFakeKafka()
		var b flam.IBroker
		k.onReader = func(*fakeReader) { _ = b.Close() }
		b = kafkaBroker(t, k, container.Bag{})
		_, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
		testutil.Error(t, err, broker.ErrClosed,
			"unable to subscribe to kafka broker topic: broker closed", map[string]any{"id": "main", "topic": "orders"})
		testutil.Equal(t, "closed", k.reader(0).isClosed(), true)
	})
}

func Test_Kafka_Client(t *testing.T) {
	ctx := context.Background()
	b := mustCreate(t, brokerkafka.Driver, container.Bag{
		brokerkafka.ConfigFieldBrokers:     []any{"127.0.0.1:1"},
		brokerkafka.ConfigFieldMaxAttempts: 1,
	}, nil)

	publishCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if err := b.Publish(publishCtx, "orders", []byte("one"), nil); !errors.Is(err, broker.ErrPublish) {
		t.Fatalf("Publish() = %v, want ErrPublish", err)
	}

	sub, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
	testutil.NoError(t, err)
	testutil.NoError(t, sub.Close())
	testutil.NoError(t, b.Close())
}
