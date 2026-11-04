package checks

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	brokerredis "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	brokercheck "github.com/cjdias/flam-in-go/pkg/flam/health/checks/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// brokerValues configures the redis broker "events", on the mini
// connection "default".
var brokerValues = map[string]any{
	redis.ConnectionConfigPath + ".default": container.Bag{factory.ConfigFieldDriver: mini.Driver},
	broker.BrokerConfigPath + ".events":     container.Bag{factory.ConfigFieldDriver: brokerredis.Driver},
}

// fakeBroker is a flam.IBroker recording the topics it publishes on, and
// failing to publish with err.
type fakeBroker struct {
	flam.IBroker

	err       error
	mu        sync.Mutex
	published []string
}

func (b *fakeBroker) Id() string   { return "fake" }
func (b *fakeBroker) Close() error { return nil }

func (b *fakeBroker) Publish(_ context.Context, topic string, payload []byte, headers map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(payload) != 0 || headers != nil {
		panic("unexpected payload or headers")
	}
	b.published = append(b.published, topic)
	return b.err
}

// topics returns the topics the broker published on.
func (b *fakeBroker) topics() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.published
}

// pingingBroker is a fakeBroker implementing flam.IHealthPinger, failing
// to ping with err.
type pingingBroker struct {
	fakeBroker

	pingErr error
	pings   int
}

func (b *pingingBroker) Ping(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pings++
	return b.pingErr
}

// brokerApp returns an app with the broker factory, the redis broker
// "events", the broker stored at "fake", unless nil, and the broker check
// driver.
func brokerApp(t *testing.T, fake flam.IBroker, values map[string]any) *testutil.App {
	t.Helper()
	all := map[string]any{}
	for path, value := range brokerValues {
		all[path] = value
	}
	for path, value := range values {
		all[path] = value
	}
	a := newApp(t, all, redis.NewBundle(), mini.NewBundle(), broker.NewBundle(), brokerredis.NewBundle(), brokercheck.NewBundle())
	t.Cleanup(func() {
		_ = a.Invoke(func(f flam.IBrokerFactory) error { return f.Close() })
		_ = a.Invoke(func(f flam.IRedisConnectionFactory) error { return f.Close() })
	})
	if fake != nil {
		testutil.NoError(t, a.Invoke(func(f flam.IBrokerFactory) error { return f.Store("fake", fake) }))
	}
	return a
}

func Test_Broker_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", brokercheck.Driver, "flam.health.checks.driver.broker")
	testutil.Equal(t, "ConfigFieldBroker", brokercheck.ConfigFieldBroker, "broker")
	testutil.Equal(t, "ConfigFieldTopic", brokercheck.ConfigFieldTopic, "topic")
	testutil.Equal(t, "ErrorFieldID", brokercheck.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", brokercheck.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldBroker", brokercheck.ErrorFieldBroker, "broker")
	testutil.Equal(t, "ErrorFieldTopic", brokercheck.ErrorFieldTopic, "topic")
}

func Test_Broker_Bundle(t *testing.T) {
	b := brokercheck.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks.broker")

	a := newApp(t, nil, b)
	c := creatorOf(t, a)
	testutil.Equal(t, "Accept(driver)", c.Accept(checkBag("events", brokercheck.Driver, nil)), true)
	testutil.Equal(t, "Accept(other)", c.Accept(checkBag("events", "other", nil)), false)
}

func Test_Broker_Create(t *testing.T) {
	t.Run("should refuse an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{health.ConfigFieldKind: "none"}, health.ConfigFieldKind},
			{container.Bag{brokercheck.ConfigFieldTopic: 1}, brokercheck.ConfigFieldTopic},
			{container.Bag{brokercheck.ConfigFieldBroker: true}, brokercheck.ConfigFieldBroker},
			{container.Bag{brokercheck.ConfigFieldBroker: ""}, brokercheck.ConfigFieldBroker},
		} {
			a := brokerApp(t, nil, nil)
			check, err := creatorOf(t, a).Create(checkBag("events", brokercheck.Driver, tc.fields))
			testutil.Error(t, err, brokercheck.ErrInvalidField,
				"unable to create broker health check: invalid broker health check field",
				map[string]any{brokercheck.ErrorFieldID: "events", brokercheck.ErrorFieldField: tc.field})
			testutil.Equal(t, "check", check, nil)
		}
	})

	t.Run("should fail without broker factory", func(t *testing.T) {
		a := newApp(t, nil, brokercheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("events", brokercheck.Driver, nil))
		testutil.Error(t, err, brokercheck.ErrNoFactory,
			"unable to create broker health check: no broker factory",
			map[string]any{brokercheck.ErrorFieldID: "events"})
	})

	t.Run("should fail for an unknown broker", func(t *testing.T) {
		a := brokerApp(t, nil, nil)
		_, err := creatorOf(t, a).Create(checkBag("jobs", brokercheck.Driver, nil))
		testutil.Error(t, err, brokercheck.ErrUnknownBroker,
			"unable to create broker health check: unknown broker",
			map[string]any{brokercheck.ErrorFieldID: "jobs", brokercheck.ErrorFieldBroker: "jobs"})
	})

	t.Run("should check the readiness unless configured", func(t *testing.T) {
		a := brokerApp(t, nil, nil)
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("events", brokercheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", check.Id(), "events")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)

		check, err = c.Create(checkBag("events", brokercheck.Driver, container.Bag{health.ConfigFieldKind: "both"}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthBoth)
	})
}

func Test_Broker_Check(t *testing.T) {
	t.Run("should take the broker without topic", func(t *testing.T) {
		a := brokerApp(t, nil, nil)
		check, err := creatorOf(t, a).Create(checkBag("events", brokercheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
		testutil.NoError(t, a.Invoke(func(f flam.IBrokerFactory) {
			testutil.Equal(t, "stored", f.Stored(), []string{"events"})
		}))
	})

	t.Run("should ping a broker of a pinging driver rather than publish", func(t *testing.T) {
		a := brokerApp(t, nil, nil)
		var b flam.IBroker
		testutil.NoError(t, a.Invoke(func(f flam.IBrokerFactory) (err error) {
			b, err = f.Get("events")
			return err
		}))
		received := make(chan flam.IBrokerMessage, 1)
		sub, err := b.Subscribe(context.Background(), "health", func(_ context.Context, msg flam.IBrokerMessage) error {
			received <- msg
			return nil
		})
		testutil.NoError(t, err)
		defer func() { _ = sub.Close() }()

		check, err := creatorOf(t, a).Create(checkBag("ready", brokercheck.Driver, container.Bag{
			brokercheck.ConfigFieldBroker: "events",
			brokercheck.ConfigFieldTopic:  "health",
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
		select {
		case msg := <-received:
			t.Fatalf("message %v published, want the redis broker pinged", msg)
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("should only take a broker of a driver not pinging without topic", func(t *testing.T) {
		fake := &fakeBroker{}
		a := brokerApp(t, fake, nil)
		check, err := creatorOf(t, a).Create(checkBag("fake", brokercheck.Driver, container.Bag{}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
		testutil.Equal(t, "topics", len(fake.topics()), 0)
	})

	t.Run("should publish on the topic of a broker of a driver not pinging", func(t *testing.T) {
		fake := &fakeBroker{}
		a := brokerApp(t, fake, nil)
		check, err := creatorOf(t, a).Create(checkBag("fake", brokercheck.Driver, container.Bag{
			brokercheck.ConfigFieldTopic: "health",
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
		testutil.Equal(t, "topics", fake.topics(), []string{"health"})
	})

	t.Run("should fail when the message cannot be published", func(t *testing.T) {
		fake := &fakeBroker{err: errBoom}
		a := brokerApp(t, fake, nil)
		check, err := creatorOf(t, a).Create(checkBag("fake", brokercheck.Driver, container.Bag{
			brokercheck.ConfigFieldTopic: "health",
		}))
		testutil.NoError(t, err)
		testutil.Error(t, check.Check(context.Background()), brokercheck.ErrPublish,
			"broker health check failed: unable to publish on broker: boom",
			map[string]any{
				brokercheck.ErrorFieldID:     "fake",
				brokercheck.ErrorFieldBroker: "fake",
				brokercheck.ErrorFieldTopic:  "health",
			})
		testutil.Equal(t, "topics", fake.topics(), []string{"health"})
	})

	t.Run("should ping a broker implementing the pinger", func(t *testing.T) {
		fake := &pingingBroker{}
		a := brokerApp(t, fake, nil)
		check, err := creatorOf(t, a).Create(checkBag("fake", brokercheck.Driver, container.Bag{
			brokercheck.ConfigFieldTopic: "health",
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
		testutil.Equal(t, "pings", fake.pings, 1)
		testutil.Equal(t, "topics", len(fake.topics()), 0)

		fake.pingErr = errBoom
		testutil.Error(t, check.Check(context.Background()), brokercheck.ErrPing,
			"broker health check failed: unable to ping broker: boom",
			map[string]any{brokercheck.ErrorFieldID: "fake", brokercheck.ErrorFieldBroker: "fake"})
	})

	t.Run("should fail when the broker cannot be created", func(t *testing.T) {
		a := brokerApp(t, nil, map[string]any{
			broker.BrokerConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "unknown"},
		})
		check, err := creatorOf(t, a).Create(checkBag("bad", brokercheck.Driver, nil))
		testutil.NoError(t, err)
		wantErr(t, check.Check(context.Background()),
			map[string]any{brokercheck.ErrorFieldID: "bad", brokercheck.ErrorFieldBroker: "bad"},
			brokercheck.ErrBroker, factory.ErrNoCreator)
	})
}
