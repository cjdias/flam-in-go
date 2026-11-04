package brokers

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	brokernats "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/nats"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// natsServer returns an embedded NATS server, with JetStream, listening on
// a random port, configured by the configure function, when not nil, and
// shut down at the end of the test.
func natsServer(t *testing.T, configure func(o *server.Options)) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      server.RANDOM_PORT,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	if configure != nil {
		configure(opts)
	}
	s, err := server.NewServer(opts)
	testutil.NoError(t, err)
	s.Start()
	if !s.ReadyForConnections(timeout) {
		t.Fatal("the nats server did not start")
	}
	t.Cleanup(s.Shutdown)
	return s
}

// natsBroker returns the NATS broker "main" of the fields, connected to
// the server.
func natsBroker(t *testing.T, s *server.Server, fields container.Bag) flam.IBroker {
	t.Helper()
	fields[brokernats.ConfigFieldURL] = s.ClientURL()
	return mustCreate(t, brokernats.Driver, fields, nil)
}

func Test_Nats_Declarations(t *testing.T) {
	b := brokernats.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.broker.brokers.nats")
	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		brokernats.DefaultURLConfigPath:            "nats://127.0.0.1:4222",
		brokernats.DefaultConnectTimeoutConfigPath: "2s",
		brokernats.DefaultReconnectWaitConfigPath:  "2s",
		brokernats.DefaultMaxReconnectsConfigPath:  60,
		brokernats.DefaultJetStreamConfigPath:      false,
		brokernats.DefaultAckWaitConfigPath:        "30s",
		brokernats.DefaultMaxDeliverConfigPath:     -1,
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}

	for _, pair := range [][2]string{
		{brokernats.Driver, "flam.broker.brokers.driver.nats"},
		{brokernats.ConfigFieldURL, "url"},
		{brokernats.ConfigFieldName, "name"},
		{brokernats.ConfigFieldToken, "token"},
		{brokernats.ConfigFieldUser, "user"},
		{brokernats.ConfigFieldPassword, "password"},
		{brokernats.ConfigFieldCredentials, "credentials"},
		{brokernats.ConfigFieldConnectTimeout, "connect_timeout"},
		{brokernats.ConfigFieldReconnectWait, "reconnect_wait"},
		{brokernats.ConfigFieldMaxReconnects, "max_reconnects"},
		{brokernats.ConfigFieldQueue, "queue"},
		{brokernats.ConfigFieldJetStream, "jetstream"},
		{brokernats.ConfigFieldStream, "stream"},
		{brokernats.ConfigFieldSubjects, "subjects"},
		{brokernats.ConfigFieldDurable, "durable"},
		{brokernats.ConfigFieldAckWait, "ack_wait"},
		{brokernats.ConfigFieldMaxDeliver, "max_deliver"},
		{brokernats.DefaultURLConfigPath, "flam.broker.defaults.brokers.nats.url"},
		{brokernats.DefaultConnectTimeoutConfigPath, "flam.broker.defaults.brokers.nats.connect_timeout"},
		{brokernats.DefaultReconnectWaitConfigPath, "flam.broker.defaults.brokers.nats.reconnect_wait"},
		{brokernats.DefaultMaxReconnectsConfigPath, "flam.broker.defaults.brokers.nats.max_reconnects"},
		{brokernats.DefaultJetStreamConfigPath, "flam.broker.defaults.brokers.nats.jetstream"},
		{brokernats.DefaultAckWaitConfigPath, "flam.broker.defaults.brokers.nats.ack_wait"},
		{brokernats.DefaultMaxDeliverConfigPath, "flam.broker.defaults.brokers.nats.max_deliver"},
		{brokernats.ErrorFieldID, "id"},
		{brokernats.ErrorFieldField, "field"},
		{brokernats.ErrorFieldURL, "url"},
		{brokernats.ErrorFieldStream, "stream"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("constant = %q, want %q", pair[0], pair[1])
		}
	}
}

func Test_Nats_Create(t *testing.T) {
	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, value := range map[string]any{
			brokernats.ConfigFieldURL:            "",
			brokernats.ConfigFieldName:           1,
			brokernats.ConfigFieldToken:          1,
			brokernats.ConfigFieldUser:           1,
			brokernats.ConfigFieldPassword:       1,
			brokernats.ConfigFieldCredentials:    1,
			brokernats.ConfigFieldConnectTimeout: "0s",
			brokernats.ConfigFieldReconnectWait:  "soon",
			brokernats.ConfigFieldMaxReconnects:  -2,
			brokernats.ConfigFieldQueue:          1,
			brokernats.ConfigFieldJetStream:      "yes",
			brokernats.ConfigFieldStream:         1,
			brokernats.ConfigFieldSubjects:       "a",
			brokernats.ConfigFieldDurable:        1,
			brokernats.ConfigFieldAckWait:        "0s",
			brokernats.ConfigFieldMaxDeliver:     -2,
		} {
			_, err := create(t, brokernats.Driver, container.Bag{field: value}, nil)
			assertInvalid(t, err, brokernats.ErrInvalidField, field)
		}
		_, err := create(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldJetStream: true}, nil)
		assertInvalid(t, err, brokernats.ErrInvalidField, brokernats.ConfigFieldStream)
	})

	t.Run("should fail when it cannot connect", func(t *testing.T) {
		s := natsServer(t, nil)
		url := s.ClientURL()
		s.Shutdown()
		_, err := create(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: url}, nil)
		var e *flam.Error
		if !errors.Is(err, brokernats.ErrConnect) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want ErrConnect", err)
		}
		testutil.Context(t, e, map[string]any{"id": "main", "url": url})

		live := natsServer(t, nil)
		_, err = create(t, brokernats.Driver, container.Bag{
			brokernats.ConfigFieldURL:         live.ClientURL(),
			brokernats.ConfigFieldCredentials: t.TempDir() + "/missing.creds",
		}, nil)
		if !errors.Is(err, brokernats.ErrConnect) {
			t.Fatalf("Get() = %v, want ErrConnect", err)
		}
	})

	t.Run("should authenticate with a token", func(t *testing.T) {
		s := natsServer(t, func(o *server.Options) { o.Authorization = "secret" })
		_, err := create(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: s.ClientURL()}, nil)
		if !errors.Is(err, brokernats.ErrConnect) {
			t.Fatalf("Get() = %v, want ErrConnect", err)
		}
		natsBroker(t, s, container.Bag{brokernats.ConfigFieldToken: "secret"})
	})

	t.Run("should authenticate with a user and a password", func(t *testing.T) {
		s := natsServer(t, func(o *server.Options) { o.Username, o.Password = "user", "pass" })
		natsBroker(t, s, container.Bag{brokernats.ConfigFieldUser: "user", brokernats.ConfigFieldPassword: "pass"})
	})

	t.Run("should fail when the stream cannot be created", func(t *testing.T) {
		s := natsServer(t, nil)
		_, err := create(t, brokernats.Driver, container.Bag{
			brokernats.ConfigFieldURL:       s.ClientURL(),
			brokernats.ConfigFieldJetStream: true,
			brokernats.ConfigFieldStream:    "in.valid",
			brokernats.ConfigFieldSubjects:  []any{"orders.>"},
		}, nil)
		var e *flam.Error
		if !errors.Is(err, brokernats.ErrStream) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want ErrStream", err)
		}
		testutil.Context(t, e, map[string]any{"id": "main", "stream": "in.valid"})
	})
}

func Test_Nats_Common(t *testing.T) {
	assertCommon(t, natsBroker(t, natsServer(t, nil), container.Bag{}), "nats")
}

func Test_Nats_Core(t *testing.T) {
	ctx := context.Background()

	t.Run("should deliver the messages to every subscription", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), container.Bag{})
		first, firstCh := collect(func(int) error { return errors.New("ignored") })
		second, secondCh := collect(nil)
		_, err := b.Subscribe(ctx, "orders.*", first)
		testutil.NoError(t, err)
		_, err = b.Subscribe(ctx, "orders.created", second)
		testutil.NoError(t, err)

		testutil.NoError(t, b.Publish(ctx, "orders.created", []byte("one"), map[string]string{"h": "v"}))
		testutil.NoError(t, b.Publish(ctx, "orders.created", []byte("two"), nil))
		want := []received{
			{topic: "orders.created", payload: "one", headers: map[string]string{"h": "v"}},
			{topic: "orders.created", payload: "two", headers: map[string]string{}},
		}
		for _, ch := range []<-chan received{firstCh, secondCh} {
			for _, w := range want {
				testutil.Equal(t, "message", next(t, ch), w)
			}
		}
	})

	t.Run("should deliver each message to one subscription of the queue group", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), container.Bag{brokernats.ConfigFieldQueue: "workers"})
		handler, ch := collect(nil)
		_, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		_, err = b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		for _, payload := range []string{"1", "2", "3"} {
			testutil.NoError(t, b.Publish(ctx, "orders", []byte(payload), nil))
		}
		got := map[string]bool{}
		for range 3 {
			got[next(t, ch).payload] = true
		}
		testutil.Equal(t, "payloads", got, map[string]bool{"1": true, "2": true, "3": true})
		none(t, ch, 50*time.Millisecond)
	})

	t.Run("should stop delivering once closed", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), container.Bag{})
		handler, ch := collect(nil)
		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, sub.Close())
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("lost"), nil))
		none(t, ch, 50*time.Millisecond)
	})

	t.Run("should wait for the handler under way once closed", func(t *testing.T) {
		for _, closer := range []string{"subscription", "broker"} {
			b := natsBroker(t, natsServer(t, nil), container.Bag{})
			started, release := make(chan struct{}), make(chan struct{})
			var returned atomic.Bool
			sub, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error {
				close(started)
				<-release
				returned.Store(true)
				return nil
			})
			testutil.NoError(t, err)
			testutil.NoError(t, b.Publish(ctx, "orders", []byte("one"), nil))
			select {
			case <-started:
			case <-time.After(timeout):
				t.Fatalf("%s: the handler was not called", closer)
			}

			closed := make(chan error, 1)
			go func() {
				if closer == "subscription" {
					closed <- sub.Close()
				} else {
					closed <- b.Close()
				}
			}()
			select {
			case err := <-closed:
				t.Fatalf("%s: Close() = %v while the handler runs", closer, err)
			case <-time.After(50 * time.Millisecond):
			}
			close(release)
			select {
			case err := <-closed:
				testutil.NoError(t, err)
			case <-time.After(timeout):
				t.Fatalf("%s: Close() did not return", closer)
			}
			testutil.Equal(t, closer+" returned", returned.Load(), true)
		}
	})

	t.Run("should fail to subscribe to an invalid subject", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), container.Bag{})
		_, err := b.Subscribe(ctx, "orders..created", func(context.Context, flam.IBrokerMessage) error { return nil })
		testutil.Error(t, err, broker.ErrSubscribe,
			"unable to subscribe to nats broker topic: unable to subscribe to broker topic: nats: invalid subject",
			map[string]any{"id": "main", "topic": "orders..created"})
	})

	t.Run("should fail once the connection is closed", func(t *testing.T) {
		s := natsServer(t, nil)
		b := natsBroker(t, s, container.Bag{brokernats.ConfigFieldMaxReconnects: 0})
		s.Shutdown()
		deadline := time.Now().Add(timeout)
		var err error
		for err == nil && time.Now().Before(deadline) {
			err = b.Publish(ctx, "orders", nil, nil)
			time.Sleep(time.Millisecond)
		}
		if !errors.Is(err, broker.ErrPublish) {
			t.Fatalf("Publish() = %v, want ErrPublish", err)
		}
		_, err = b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
		if !errors.Is(err, broker.ErrSubscribe) {
			t.Fatalf("Subscribe() = %v, want ErrSubscribe", err)
		}
	})
}

func Test_Nats_JetStream(t *testing.T) {
	ctx := context.Background()
	jetStream := func(fields container.Bag) container.Bag {
		fields[brokernats.ConfigFieldJetStream] = true
		fields[brokernats.ConfigFieldStream] = "ORDERS"
		fields[brokernats.ConfigFieldSubjects] = []any{"orders.>"}
		return fields
	}

	t.Run("should deliver the messages stored once subscribed", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), jetStream(container.Bag{}))
		testutil.NoError(t, b.Publish(ctx, "orders.created", []byte("before"), nil))
		handler, ch := collect(nil)
		sub, err := b.Subscribe(ctx, "orders.created", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders.created", []byte("after"), map[string]string{"h": "v"}))
		testutil.Equal(t, "message", next(t, ch),
			received{id: "2", topic: "orders.created", payload: "after", headers: map[string]string{"h": "v"}})
		none(t, ch, 50*time.Millisecond)
		testutil.NoError(t, sub.Close())
	})

	t.Run("should deliver again the failed messages", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), jetStream(container.Bag{brokernats.ConfigFieldDurable: "workers"}))
		handler, ch := collect(func(n int) error {
			if n == 1 {
				return errors.New("failed")
			}
			return nil
		})
		_, err := b.Subscribe(ctx, "orders.created", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders.created", []byte("retried"), nil))
		first, second := next(t, ch), next(t, ch)
		testutil.Equal(t, "redelivered", second, first)
		none(t, ch, 50*time.Millisecond)
	})

	t.Run("should give each topic a consumer of its own", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), jetStream(container.Bag{}))
		topics := []string{"orders.a_b", "orders.a.b", "orders.*", "orders.>"}
		chans := make([]<-chan received, len(topics))
		for i, topic := range topics {
			var handler flam.BrokerHandler
			handler, chans[i] = collect(nil)
			_, err := b.Subscribe(ctx, topic, handler)
			testutil.NoError(t, err)
		}
		testutil.NoError(t, b.Publish(ctx, "orders.a_b", []byte("underscore"), nil))
		testutil.NoError(t, b.Publish(ctx, "orders.a.b", []byte("dot"), nil))
		for i, want := range [][]string{{"underscore"}, {"dot"}, {"underscore"}, {"underscore", "dot"}} {
			for _, payload := range want {
				got := next(t, chans[i])
				testutil.Equal(t, topics[i]+" payload", got.payload, payload)
			}
			none(t, chans[i], 50*time.Millisecond)
		}
	})

	t.Run("should consume the topics too long for a consumer name", func(t *testing.T) {
		b := natsBroker(t, natsServer(t, nil), jetStream(container.Bag{}))
		long := "orders.x" + strings.Repeat("é", 60) // cut within a rune
		longer := long + "y"
		handler, ch := collect(nil)
		_, err := b.Subscribe(ctx, long, handler)
		testutil.NoError(t, err)
		other, otherCh := collect(nil)
		_, err = b.Subscribe(ctx, longer, other)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, long, []byte("one"), nil))
		testutil.NoError(t, b.Publish(ctx, longer, []byte("two"), nil))
		testutil.Equal(t, "long", next(t, ch), received{id: "1", topic: long, payload: "one", headers: map[string]string{}})
		testutil.Equal(t, "longer", next(t, otherCh), received{id: "2", topic: longer, payload: "two", headers: map[string]string{}})
		none(t, ch, 50*time.Millisecond)
		none(t, otherCh, 0)
	})

	t.Run("should use the stream as it exists without subjects", func(t *testing.T) {
		s := natsServer(t, nil)
		natsBroker(t, s, jetStream(container.Bag{}))
		b := natsBroker(t, s, container.Bag{brokernats.ConfigFieldJetStream: true, brokernats.ConfigFieldStream: "ORDERS"})
		handler, ch := collect(nil)
		_, err := b.Subscribe(ctx, "orders.created", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders.created", []byte("one"), nil))
		testutil.Equal(t, "payload", next(t, ch).payload, "one")
	})

	t.Run("should fail outside of the stream", func(t *testing.T) {
		s := natsServer(t, nil)
		b := natsBroker(t, s, jetStream(container.Bag{}))
		err := b.Publish(ctx, "other", nil, nil)
		if !errors.Is(err, broker.ErrPublish) {
			t.Fatalf("Publish() = %v, want ErrPublish", err)
		}
		missing := natsBroker(t, s, container.Bag{brokernats.ConfigFieldJetStream: true, brokernats.ConfigFieldStream: "MISSING"})
		_, err = missing.Subscribe(ctx, "orders.created", func(context.Context, flam.IBrokerMessage) error { return nil })
		if !errors.Is(err, broker.ErrSubscribe) {
			t.Fatalf("Subscribe() = %v, want ErrSubscribe", err)
		}
	})
}
