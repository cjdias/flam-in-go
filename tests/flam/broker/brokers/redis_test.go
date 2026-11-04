package brokers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	brokerredis "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// redisBroker returns the redis broker "main" of the fields, along with
// the redis connection "default" it uses and its server.
func redisBroker(t *testing.T, fields container.Bag) (flam.IBroker, flam.IRedisConnection, *miniredis.Miniredis) {
	t.Helper()
	a := newApp(t, brokerredis.Driver, fields, nil)
	b, err := brokerFactory(t, a).Get("main")
	testutil.NoError(t, err)
	var conn flam.IRedisConnection
	testutil.NoError(t, a.Invoke(func(f flam.IRedisConnectionFactory) (err error) {
		conn, err = f.Get("default")
		return err
	}))
	server, _ := mini.Server(conn)
	return b, conn, server
}

func Test_Redis_Declarations(t *testing.T) {
	b := brokerredis.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.broker.brokers.redis")
	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		brokerredis.DefaultConnectionConfigPath: "default",
		brokerredis.DefaultModeConfigPath:       "pubsub",
		brokerredis.DefaultStartConfigPath:      "new",
		brokerredis.DefaultBatchConfigPath:      10,
		brokerredis.DefaultBlockConfigPath:      "1s",
		brokerredis.DefaultClaimIdleConfigPath:  "30s",
		brokerredis.DefaultMaxLenConfigPath:     0,
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}

	for _, pair := range [][2]string{
		{brokerredis.Driver, "flam.broker.brokers.driver.redis"},
		{brokerredis.ModePubSub, "pubsub"},
		{brokerredis.ModeStreams, "streams"},
		{brokerredis.StartNew, "new"},
		{brokerredis.StartAll, "all"},
		{brokerredis.ConfigFieldConnection, "connection"},
		{brokerredis.ConfigFieldMode, "mode"},
		{brokerredis.ConfigFieldGroup, "group"},
		{brokerredis.ConfigFieldConsumer, "consumer"},
		{brokerredis.ConfigFieldStart, "start"},
		{brokerredis.ConfigFieldBatch, "batch"},
		{brokerredis.ConfigFieldBlock, "block"},
		{brokerredis.ConfigFieldClaimIdle, "claim_idle"},
		{brokerredis.ConfigFieldMaxLen, "max_len"},
		{brokerredis.DefaultConnectionConfigPath, "flam.broker.defaults.brokers.redis.connection"},
		{brokerredis.DefaultModeConfigPath, "flam.broker.defaults.brokers.redis.mode"},
		{brokerredis.DefaultStartConfigPath, "flam.broker.defaults.brokers.redis.start"},
		{brokerredis.DefaultBatchConfigPath, "flam.broker.defaults.brokers.redis.batch"},
		{brokerredis.DefaultBlockConfigPath, "flam.broker.defaults.brokers.redis.block"},
		{brokerredis.DefaultClaimIdleConfigPath, "flam.broker.defaults.brokers.redis.claim_idle"},
		{brokerredis.DefaultMaxLenConfigPath, "flam.broker.defaults.brokers.redis.max_len"},
		{brokerredis.StreamFieldPayload, "payload"},
		{brokerredis.StreamFieldHeaderPrefix, "header."},
		{brokerredis.ErrorFieldID, "id"},
		{brokerredis.ErrorFieldField, "field"},
		{brokerredis.ErrorFieldConnection, "connection"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("constant = %q, want %q", pair[0], pair[1])
		}
	}
}

func Test_Redis_Create(t *testing.T) {
	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, value := range map[string]any{
			brokerredis.ConfigFieldConnection: "",
			brokerredis.ConfigFieldMode:       "other",
			brokerredis.ConfigFieldGroup:      1,
			brokerredis.ConfigFieldConsumer:   1,
			brokerredis.ConfigFieldStart:      1,
			brokerredis.ConfigFieldBatch:      0,
			brokerredis.ConfigFieldBlock:      "0s",
			brokerredis.ConfigFieldClaimIdle:  "-1s",
			brokerredis.ConfigFieldMaxLen:     -1,
		} {
			_, err := create(t, brokerredis.Driver, container.Bag{field: value}, nil)
			assertInvalid(t, err, brokerredis.ErrInvalidField, field)
		}
	})

	t.Run("should fail with an unknown connection", func(t *testing.T) {
		_, err := create(t, brokerredis.Driver, container.Bag{brokerredis.ConfigFieldConnection: "missing"}, nil)
		var e *flam.Error
		if !errors.Is(err, brokerredis.ErrConnection) || !errors.Is(err, factory.ErrUnknownID) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want ErrConnection", err)
		}
		testutil.Context(t, e, map[string]any{"id": "main", "connection": "missing"})
	})

	t.Run("should fail without the redis bundle", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, bundle.NewRegisterer(a).
			QueueBundle(config.NewBundle()).
			QueueBundle(broker.NewBundle()).
			QueueBundle(brokerredis.NewBundle()).
			Register())
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(broker.BrokerConfigPath+".main", container.Bag{factory.ConfigFieldDriver: brokerredis.Driver}))
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
		_, err := brokerFactory(t, a).Get("main")
		var e *flam.Error
		if !errors.Is(err, brokerredis.ErrConnection) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want ErrConnection", err)
		}
		testutil.Equal(t, "error", e.Error(), "unable to create redis broker: unable to get redis broker connection: "+
			"no redis connection factory, register the redis bundle")
		testutil.Context(t, e, map[string]any{"id": "main", "connection": "default"})
	})
}

func Test_Redis_Common(t *testing.T) {
	for _, mode := range []string{brokerredis.ModePubSub, brokerredis.ModeStreams} {
		t.Run(mode, func(t *testing.T) {
			b, _, _ := redisBroker(t, container.Bag{brokerredis.ConfigFieldMode: mode, brokerredis.ConfigFieldBlock: "20ms"})
			assertCommon(t, b, "redis")
		})
	}
}

func Test_Redis_PubSub(t *testing.T) {
	ctx := context.Background()

	t.Run("should deliver the messages to every subscription", func(t *testing.T) {
		b, conn, _ := redisBroker(t, container.Bag{})
		first, firstCh := collect(func(int) error { return errors.New("ignored") })
		second, secondCh := collect(nil)
		_, err := b.Subscribe(ctx, "orders", first)
		testutil.NoError(t, err)
		_, err = b.Subscribe(ctx, "orders", second)
		testutil.NoError(t, err)

		testutil.NoError(t, b.Publish(ctx, "orders", []byte("one"), map[string]string{"h": "v"}))
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("two"), nil))
		want := []received{
			{topic: "orders", payload: "one", headers: map[string]string{"h": "v"}},
			{topic: "orders", payload: "two", headers: map[string]string{}},
		}
		for _, ch := range []<-chan received{firstCh, secondCh} {
			for _, w := range want {
				testutil.Equal(t, "message", next(t, ch), w)
			}
		}

		for _, foreign := range []string{`{"other":1}`, `{"payload":1}`, `not json`} {
			testutil.NoError(t, conn.Publish(ctx, "orders", foreign).Err())
			testutil.Equal(t, "foreign message", next(t, secondCh),
				received{topic: "orders", payload: foreign, headers: map[string]string{}})
		}
	})

	t.Run("should deliver a nil payload as an empty one with its headers", func(t *testing.T) {
		b, conn, _ := redisBroker(t, container.Bag{})
		handler, ch := collect(nil)
		_, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)

		testutil.NoError(t, b.Publish(ctx, "orders", nil, map[string]string{"h": "v"}))
		testutil.Equal(t, "message", next(t, ch), received{topic: "orders", payload: "", headers: map[string]string{"h": "v"}})
		testutil.NoError(t, b.Publish(ctx, "orders", []byte{}, nil))
		testutil.Equal(t, "message", next(t, ch), received{topic: "orders", payload: "", headers: map[string]string{}})
		testutil.NoError(t, conn.Publish(ctx, "orders", `{"headers":{"h":"v"},"payload":null}`).Err())
		testutil.Equal(t, "null payload", next(t, ch), received{topic: "orders", payload: "", headers: map[string]string{"h": "v"}})
	})

	t.Run("should end once its connection is closed", func(t *testing.T) {
		b, conn, _ := redisBroker(t, container.Bag{})
		sub, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
		testutil.NoError(t, err)
		_ = conn.Close() // the factory closes it again at the end of the test

		err = ended(t, sub)
		testutil.Error(t, err, broker.ErrReceive,
			"unable to receive redis broker message: unable to receive broker message: redis broker channel closed",
			map[string]any{"id": "main", "topic": "orders"})
	})

	t.Run("should stop delivering once closed or once its context is done", func(t *testing.T) {
		b, _, _ := redisBroker(t, container.Bag{})
		handler, ch := collect(nil)
		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, sub.Close())
		testutil.NoError(t, sub.Close())

		subCtx, cancel := context.WithCancel(ctx)
		other, err := b.Subscribe(subCtx, "orders", handler)
		testutil.NoError(t, err)
		cancel()
		testutil.NoError(t, ended(t, other))

		testutil.NoError(t, b.Publish(ctx, "orders", []byte("lost"), nil))
		none(t, ch, 50*time.Millisecond)
	})

	t.Run("should recover the handler panics", func(t *testing.T) {
		b, _, _ := redisBroker(t, container.Bag{})
		ch := make(chan string, 2)
		_, err := b.Subscribe(ctx, "orders", func(_ context.Context, msg flam.IBrokerMessage) error {
			ch <- string(msg.Payload())
			if string(msg.Payload()) == "error" {
				panic(errors.New("boom"))
			}
			panic("boom")
		})
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("error"), nil))
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("value"), nil))
		testutil.Equal(t, "first", <-ch, "error")
		testutil.Equal(t, "second", <-ch, "value")
	})

	t.Run("should fail when the server is unreachable", func(t *testing.T) {
		b, _, server := redisBroker(t, container.Bag{})
		server.Close()

		err := b.Publish(ctx, "orders", nil, nil)
		if !errors.Is(err, broker.ErrPublish) {
			t.Fatalf("Publish() = %v, want ErrPublish", err)
		}
		_, err = b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
		if !errors.Is(err, broker.ErrSubscribe) {
			t.Fatalf("Subscribe() = %v, want ErrSubscribe", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "main", "topic": "orders"})
	})
}

func Test_Redis_Streams(t *testing.T) {
	ctx := context.Background()
	streams := func(fields container.Bag) container.Bag {
		fields[brokerredis.ConfigFieldMode] = brokerredis.ModeStreams
		fields[brokerredis.ConfigFieldBlock] = "20ms"
		return fields
	}

	t.Run("should deliver the messages added from the creation of the group", func(t *testing.T) {
		b, conn, _ := redisBroker(t, streams(container.Bag{brokerredis.ConfigFieldMaxLen: 1}))
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("before"), nil))

		handler, ch := collect(nil)
		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("after"), map[string]string{"h": "v"}))
		got := next(t, ch)
		if got.id == "" {
			t.Error("the message has no id")
		}
		got.id = ""
		testutil.Equal(t, "message", got, received{topic: "orders", payload: "after", headers: map[string]string{"h": "v"}})
		none(t, ch, 50*time.Millisecond)
		testutil.NoError(t, sub.Close())

		pending, err := conn.XPending(ctx, "orders", "main").Result()
		testutil.NoError(t, err)
		testutil.Equal(t, "pending", pending.Count, int64(0))
	})

	t.Run("should deliver the whole stream from the start", func(t *testing.T) {
		b, _, _ := redisBroker(t, streams(container.Bag{
			brokerredis.ConfigFieldStart:    brokerredis.StartAll,
			brokerredis.ConfigFieldGroup:    "workers",
			brokerredis.ConfigFieldConsumer: "worker",
		}))
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("before"), nil))
		handler, ch := collect(nil)
		_, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.Equal(t, "payload", next(t, ch).payload, "before")
	})

	t.Run("should share the messages between the subscriptions of the group", func(t *testing.T) {
		b, _, _ := redisBroker(t, streams(container.Bag{}))
		handler, ch := collect(nil)
		_, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		_, err = b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		for _, payload := range []string{"1", "2", "3", "4"} {
			testutil.NoError(t, b.Publish(ctx, "orders", []byte(payload), nil))
		}
		got := map[string]bool{}
		for range 4 {
			got[next(t, ch).payload] = true
		}
		testutil.Equal(t, "payloads", got, map[string]bool{"1": true, "2": true, "3": true, "4": true})
		none(t, ch, 50*time.Millisecond)
	})

	t.Run("should delete its own consumer once closed, unless it holds pending entries", func(t *testing.T) {
		b, conn, _ := redisBroker(t, streams(container.Bag{brokerredis.ConfigFieldClaimIdle: "1h"}))
		handler, ch := collect(func(n int) error {
			if n == 2 {
				return errors.New("failed")
			}
			return nil
		})
		consumers := func() []goredis.XInfoConsumer {
			t.Helper()
			list, err := conn.XInfoConsumers(ctx, "orders", "main").Result()
			testutil.NoError(t, err)
			return list
		}

		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("handled"), nil))
		testutil.Equal(t, "payload", next(t, ch).payload, "handled")
		testutil.Equal(t, "consumers", len(consumers()), 1)
		testutil.NoError(t, sub.Close())
		testutil.Equal(t, "consumers once closed", len(consumers()), 0)

		sub, err = b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("failed"), nil))
		testutil.Equal(t, "payload", next(t, ch).payload, "failed")
		testutil.NoError(t, sub.Close())
		kept := consumers()
		testutil.Equal(t, "consumers with pending entries", len(kept), 1)
		testutil.Equal(t, "pending", kept[0].Pending, int64(1))
	})

	t.Run("should keep the configured consumer once closed", func(t *testing.T) {
		b, conn, _ := redisBroker(t, streams(container.Bag{brokerredis.ConfigFieldConsumer: "worker"}))
		handler, ch := collect(nil)
		sub, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("handled"), nil))
		testutil.Equal(t, "payload", next(t, ch).payload, "handled")
		testutil.NoError(t, sub.Close())
		list, err := conn.XInfoConsumers(ctx, "orders", "main").Result()
		testutil.NoError(t, err)
		testutil.Equal(t, "consumers", len(list), 1)
		testutil.Equal(t, "name", list[0].Name, "worker")
	})

	t.Run("should deliver again the failed messages once idle", func(t *testing.T) {
		b, _, _ := redisBroker(t, streams(container.Bag{brokerredis.ConfigFieldClaimIdle: "10ms"}))
		handler, ch := collect(func(n int) error {
			if n == 1 {
				return errors.New("failed")
			}
			return nil
		})
		_, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("retried"), nil))
		first, second := next(t, ch), next(t, ch)
		testutil.Equal(t, "redelivered", second, first)
		none(t, ch, 50*time.Millisecond)
	})

	t.Run("should leave the rest of a batch pending once its context is done", func(t *testing.T) {
		b, conn, _ := redisBroker(t, streams(container.Bag{brokerredis.ConfigFieldStart: brokerredis.StartAll}))
		for _, payload := range []string{"1", "2"} {
			testutil.NoError(t, b.Publish(ctx, "orders", []byte(payload), nil))
		}
		subCtx, cancel := context.WithCancel(ctx)
		sub, err := b.Subscribe(subCtx, "orders", func(context.Context, flam.IBrokerMessage) error {
			cancel()
			return nil
		})
		testutil.NoError(t, err)
		testutil.NoError(t, ended(t, sub))
		pending, err := conn.XPending(ctx, "orders", "main").Result()
		testutil.NoError(t, err)
		testutil.Equal(t, "pending", pending.Count, int64(2))
	})

	t.Run("should end without error once its context is done while claiming", func(t *testing.T) {
		b, _, _ := redisBroker(t, streams(container.Bag{
			brokerredis.ConfigFieldStart:     brokerredis.StartAll,
			brokerredis.ConfigFieldClaimIdle: "10ms",
		}))
		testutil.NoError(t, b.Publish(ctx, "orders", []byte("1"), nil))
		subCtx, cancel := context.WithCancel(ctx)
		n := 0
		sub, err := b.Subscribe(subCtx, "orders", func(context.Context, flam.IBrokerMessage) error {
			n++
			if n == 1 {
				return errors.New("failed")
			}
			cancel()
			return nil
		})
		testutil.NoError(t, err)
		testutil.NoError(t, ended(t, sub))
	})

	t.Run("should end when the stream cannot be read", func(t *testing.T) {
		for _, claimIdle := range []string{"0s", "1h"} {
			b, conn, _ := redisBroker(t, streams(container.Bag{brokerredis.ConfigFieldClaimIdle: claimIdle}))
			sub, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
			testutil.NoError(t, err)
			testutil.NoError(t, conn.Del(ctx, "orders").Err())
			testutil.NoError(t, conn.Set(ctx, "orders", "not a stream", 0).Err())
			err = ended(t, sub)
			if !errors.Is(err, broker.ErrReceive) || !strings.Contains(err.Error(), "unable to receive redis broker message") {
				t.Fatalf("Err() = %v, want ErrReceive", err)
			}
			testutil.Context(t, err.(*flam.Error), map[string]any{"id": "main", "topic": "orders"})
			testutil.NoError(t, sub.Close())
		}
	})

	t.Run("should fail to subscribe to a key that is not a stream", func(t *testing.T) {
		b, conn, _ := redisBroker(t, streams(container.Bag{}))
		testutil.NoError(t, conn.Set(ctx, "orders", "not a stream", 0).Err())
		_, err := b.Subscribe(ctx, "orders", func(context.Context, flam.IBrokerMessage) error { return nil })
		if !errors.Is(err, broker.ErrSubscribe) {
			t.Fatalf("Subscribe() = %v, want ErrSubscribe", err)
		}
		err = b.Publish(ctx, "orders", nil, nil)
		if !errors.Is(err, broker.ErrPublish) {
			t.Fatalf("Publish() = %v, want ErrPublish", err)
		}
	})
}
