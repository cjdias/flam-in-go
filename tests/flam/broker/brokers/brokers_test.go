package brokers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// timeout bounds the waits of the tests.
const timeout = 5 * time.Second

// newApp returns a testutil.App with the config, redis, mini connection,
// broker and broker drivers bundles, the redis connection "default" of
// the mini driver, the broker "main" of the driver and the fields, and
// the values, after calling the setups, such as to provide a kafka
// client.
func newApp(t *testing.T, driver string, fields container.Bag, values map[string]any, setups ...func(a *testutil.App)) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), redis.NewBundle(), mini.NewBundle(), broker.NewBundle(), brokers.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(redis.ConnectionConfigPath+".default", container.Bag{factory.ConfigFieldDriver: mini.Driver}))
	fields[factory.ConfigFieldDriver] = driver
	testutil.NoError(t, bag.Set(broker.BrokerConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	a := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	for _, setup := range setups {
		setup(a)
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = a.Invoke(func(f flam.IBrokerFactory) error { return f.Close() })
		_ = a.Invoke(func(f flam.IRedisConnectionFactory) error { return f.Close() })
	})
	return a
}

// brokerFactory returns the broker factory of the app.
func brokerFactory(t *testing.T, a *testutil.App) flam.IBrokerFactory {
	t.Helper()
	var f flam.IBrokerFactory
	testutil.NoError(t, a.Invoke(func(provided flam.IBrokerFactory) { f = provided }))
	return f
}

// create returns the broker "main" of the driver and the fields.
func create(t *testing.T, driver string, fields container.Bag, values map[string]any, setups ...func(a *testutil.App)) (flam.IBroker, error) {
	t.Helper()
	return brokerFactory(t, newApp(t, driver, fields, values, setups...)).Get("main")
}

// mustCreate returns the broker of create, failing the test on error.
func mustCreate(t *testing.T, driver string, fields container.Bag, values map[string]any, setups ...func(a *testutil.App)) flam.IBroker {
	t.Helper()
	b, err := create(t, driver, fields, values, setups...)
	testutil.NoError(t, err)
	testutil.Equal(t, "Id()", b.Id(), "main")
	return b
}

// received is a message handed to a collect handler.
type received struct {
	id      string
	topic   string
	payload string
	headers map[string]string
}

// collect returns a handler sending the messages it receives to the
// channel, returned along with it, then returning the error of fail for
// the number of the message, from 1, when fail is not nil.
func collect(fail func(n int) error) (flam.BrokerHandler, <-chan received) {
	ch := make(chan received, 100)
	var n atomic.Int64
	return func(_ context.Context, msg flam.IBrokerMessage) error {
		count := int(n.Add(1))
		ch <- received{id: msg.Id(), topic: msg.Topic(), payload: string(msg.Payload()), headers: msg.Headers()}
		if fail != nil {
			return fail(count)
		}
		return nil
	}, ch
}

// next returns the next message of the channel, failing the test when
// none comes in time.
func next(t *testing.T, ch <-chan received) received {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		t.Fatal("no message received")
		return received{}
	}
}

// none fails the test when a message comes within the delay.
func none(t *testing.T, ch <-chan received, delay time.Duration) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("unexpected message %#v", r)
	case <-time.After(delay):
	}
}

// ended waits for the end of the subscription, failing the test when it
// does not end in time, and returns its error.
func ended(t *testing.T, sub flam.IBrokerSubscription) error {
	t.Helper()
	select {
	case <-sub.Done():
		return sub.Err()
	case <-time.After(timeout):
		t.Fatal("the subscription did not end")
		return nil
	}
}

// assertInvalid asserts that the creation failed with the sentinel for
// the field.
func assertInvalid(t *testing.T, err, sentinel error, field string) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(errors.Unwrap(err), &e) {
		t.Fatalf("%s: Get() = %v, want %v", field, err, sentinel)
	}
	testutil.Context(t, e, map[string]any{"id": "main", "field": field})
}

// assertCommon asserts the behaviour every driver shares: the checks of
// the topic and of the handler, and the refusal of a closed broker.
func assertCommon(t *testing.T, b flam.IBroker, driver string) {
	t.Helper()
	ctx := context.Background()
	publishText := "unable to publish " + driver + " broker message: "
	subscribeText := "unable to subscribe to " + driver + " broker topic: "

	testutil.Error(t, b.Publish(ctx, "", nil, nil), broker.ErrInvalidTopic,
		publishText+"invalid broker topic", map[string]any{"id": "main", "topic": ""})
	_, err := b.Subscribe(ctx, "", func(context.Context, flam.IBrokerMessage) error { return nil })
	testutil.Error(t, err, broker.ErrInvalidTopic,
		subscribeText+"invalid broker topic", map[string]any{"id": "main", "topic": ""})
	_, err = b.Subscribe(ctx, "a", nil)
	testutil.Error(t, err, broker.ErrNilHandler,
		subscribeText+"nil broker handler", map[string]any{"id": "main", "topic": "a"})

	//lint:ignore SA1012 a nil context, never done, is what is tested
	sub, err := b.Subscribe(nil, "a", func(context.Context, flam.IBrokerMessage) error { return nil }) //nolint:staticcheck // as above
	testutil.NoError(t, err)
	testutil.Equal(t, "Topic()", sub.Topic(), "a")
	testutil.NoError(t, sub.Err())

	testutil.NoError(t, b.Close())
	testutil.NoError(t, ended(t, sub))
	testutil.NoError(t, b.Close())
	testutil.Error(t, b.Publish(ctx, "a", nil, nil), broker.ErrClosed,
		publishText+"broker closed", map[string]any{"id": "main", "topic": "a"})
	_, err = b.Subscribe(ctx, "a", func(context.Context, flam.IBrokerMessage) error { return nil })
	testutil.Error(t, err, broker.ErrClosed,
		subscribeText+"broker closed", map[string]any{"id": "main", "topic": "a"})
}

func Test_AggregateBundle(t *testing.T) {
	b := brokers.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.broker.brokers")
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(b))
	testutil.Equal(t, "calls", a.Calls(), []string{
		"bundle:flam.broker.brokers",
		"bundle:flam.broker.brokers.redis", "provider/1",
		"bundle:flam.broker.brokers.nats", "provider/1",
		"bundle:flam.broker.brokers.kafka", "provider/1",
	})
}

func Test_OtherDriver(t *testing.T) {
	_, err := create(t, "other", container.Bag{}, nil)
	if !errors.Is(err, factory.ErrNoCreator) {
		t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
	}
}
