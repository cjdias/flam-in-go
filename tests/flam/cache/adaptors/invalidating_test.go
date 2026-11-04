package adaptors

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	brokerredis "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/invalidating"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamredis "github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newInvalidatingApp returns a testutil.App with the config, cache, json,
// field, memory and invalidating bundles, the broker ones and a redis
// broker "main" in the pubsub mode over a mini connection "default",
// unless without, a fake clock, the memory adaptors "local1" and
// "local2", and the values.
func newInvalidatingApp(t *testing.T, values map[string]any, withBroker bool) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{
		config.NewBundle(), cache.NewBundle(), json.NewBundle(), field.NewBundle(),
		memory.NewBundle(), invalidating.NewBundle(),
	}
	if withBroker {
		bundles = append(bundles, flamredis.NewBundle(), mini.NewBundle(), broker.NewBundle(), brokerredis.NewBundle())
	}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(flamredis.ConnectionConfigPath+".default", container.Bag{factory.ConfigFieldDriver: mini.Driver}))
	testutil.NoError(t, bag.Set(broker.BrokerConfigPath+".main", container.Bag{factory.ConfigFieldDriver: brokerredis.Driver}))
	for _, id := range []string{"local1", "local2"} {
		testutil.NoError(t, bag.Set(cache.AdaptorConfigPath+"."+id, container.Bag{factory.ConfigFieldDriver: memory.Driver}))
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	clock := testutil.NewClock(memoryNow)
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = cache.NewBundle().(flam.IClosableBundle).Close(app)
		if withBroker {
			_ = broker.NewBundle().(flam.IClosableBundle).Close(app)
			_ = flamredis.NewBundle().(flam.IClosableBundle).Close(app)
		}
	})
	return app
}

// invalidatingAdaptors returns the adaptors of the ids, created by the
// factory of the app, failing on the first error.
func invalidatingAdaptors(t *testing.T, app *testutil.App, ids ...string) ([]flam.ICacheAdaptor, error) {
	t.Helper()
	var adaptors []flam.ICacheAdaptor
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ICacheAdaptorFactory) {
		for _, id := range ids {
			var a flam.ICacheAdaptor
			if a, err = f.Get(id); err != nil {
				return
			}
			adaptors = append(adaptors, a)
		}
	}))
	return adaptors, err
}

// invalidatingAdaptor returns the invalidating adaptor "main" of the
// fields, created by the factory of an app with the broker unless
// without, and the values.
func invalidatingAdaptor(t *testing.T, fields container.Bag, values map[string]any, withBroker bool) (flam.ICacheAdaptor, error) {
	t.Helper()
	fields[factory.ConfigFieldDriver] = invalidating.Driver
	all := map[string]any{cache.AdaptorConfigPath + ".main": fields}
	for path, value := range values {
		all[path] = value
	}
	adaptors, err := invalidatingAdaptors(t, newInvalidatingApp(t, all, withBroker), "main")
	if err != nil {
		return nil, err
	}
	return adaptors[0], nil
}

// appBroker returns the broker "main" of the app.
func appBroker(t *testing.T, app *testutil.App) flam.IBroker {
	t.Helper()
	var b flam.IBroker
	testutil.NoError(t, app.Invoke(func(f flam.IBrokerFactory) (err error) {
		b, err = f.Get("main")
		return err
	}))
	return b
}

// eventually fails the test unless the condition holds within 5 seconds.
func eventually(t *testing.T, name string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: condition not met in time", name)
		}
		time.Sleep(time.Millisecond)
	}
}

// fakeBroker records the messages published, failing with publishErr,
// and holds the handler of its only subscription, failing with
// subscribeErr.
type fakeBroker struct {
	flam.IBroker

	subscribeErr error
	publishErr   error

	mu        sync.Mutex
	topic     string
	handler   flam.BrokerHandler
	sub       *fakeSubscription
	published []flam.IBrokerMessage
}

func (b *fakeBroker) Id() string { return "fake" }

func (b *fakeBroker) Subscribe(_ context.Context, topic string, handler flam.BrokerHandler) (flam.IBrokerSubscription, error) {
	if b.subscribeErr != nil {
		return nil, b.subscribeErr
	}
	b.topic, b.handler, b.sub = topic, handler, &fakeSubscription{}
	return b.sub, nil
}

func (b *fakeBroker) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	if b.publishErr != nil {
		return b.publishErr
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.published = append(b.published, broker.NewMessage("", topic, payload, headers))
	return nil
}

// payloads returns the payloads of the messages published.
func (b *fakeBroker) payloads() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	payloads := []string{}
	for _, msg := range b.published {
		payloads = append(payloads, string(msg.Payload()))
	}
	return payloads
}

// fakeSubscription counts its Close calls, failing with err.
type fakeSubscription struct {
	flam.IBrokerSubscription

	closes int
	err    error
}

func (s *fakeSubscription) Close() error {
	s.closes++
	return s.err
}

// recorder records the keys given to the DelKeys method of its adaptor.
type recorder struct {
	flam.ICacheKeyedAdaptor

	mu      sync.Mutex
	deleted [][]string
}

func (r *recorder) DelKeys(ctx context.Context, keys ...string) error {
	r.mu.Lock()
	r.deleted = append(r.deleted, keys)
	r.mu.Unlock()
	return r.ICacheKeyedAdaptor.DelKeys(ctx, keys...)
}

// calls returns the keys given to DelKeys.
func (r *recorder) calls() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]string{}, r.deleted...)
}

// newFakeInvalidating returns the invalidating adaptor "main" over a
// memory adaptor and a fake broker, publishing the keys set unless
// without.
func newFakeInvalidating(t *testing.T, publishSet bool) (flam.ICacheKeyedAdaptor, flam.ICacheAdaptor, *fakeBroker) {
	t.Helper()
	local, _ := newMemoryAdaptor(t, 10, 0)
	b := &fakeBroker{}
	a, err := invalidating.NewAdaptor("main", local, b, "topic", publishSet)
	testutil.NoError(t, err)
	return a, local, b
}

func Test_InvalidatingBundle(t *testing.T) {
	t.Run("should declare the driver, fields, paths and defaults", func(t *testing.T) {
		for _, pair := range [][2]string{
			{invalidating.Driver, "flam.cache.adaptors.driver.invalidating"},
			{invalidating.ConfigFieldAdaptor, "adaptor"},
			{invalidating.ConfigFieldBroker, "broker"},
			{invalidating.ConfigFieldTopic, "topic"},
			{invalidating.ConfigFieldPublishSet, "publish_set"},
			{invalidating.DefaultAdaptorConfigPath, "flam.cache.defaults.adaptors.invalidating.adaptor"},
			{invalidating.DefaultBrokerConfigPath, "flam.cache.defaults.adaptors.invalidating.broker"},
			{invalidating.DefaultTopicConfigPath, "flam.cache.defaults.adaptors.invalidating.topic"},
			{invalidating.DefaultPublishSetConfigPath, "flam.cache.defaults.adaptors.invalidating.publish_set"},
			{invalidating.TopicPrefix, "flam.cache.invalidations."},
			{invalidating.HeaderInstance, "flam-cache-instance"},
			{invalidating.ErrorFieldID, "id"},
			{invalidating.ErrorFieldField, "field"},
			{invalidating.ErrorFieldAdaptor, "adaptor"},
			{invalidating.ErrorFieldBroker, "broker"},
			{invalidating.ErrorFieldTopic, "topic"},
		} {
			if pair[0] != pair[1] {
				t.Errorf("constant = %q, want %q", pair[0], pair[1])
			}
		}
		testutil.Equal(t, "DefaultPublishSet", invalidating.DefaultPublishSet, true)
	})

	t.Run("should provide the creator", func(t *testing.T) {
		b := invalidating.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.adaptors.invalidating")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.cache.adaptors.invalidating", "provider/1"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := invalidating.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		app := newInvalidatingApp(t, nil, false)
		_, err := invalidatingAdaptors(t, app, "local1")
		testutil.NoError(t, err)
	})
}

func Test_InvalidatingCreator(t *testing.T) {
	t.Run("should create the adaptor with the built-in defaults", func(t *testing.T) {
		app := newInvalidatingApp(t, map[string]any{
			cache.AdaptorConfigPath + ".main": container.Bag{
				factory.ConfigFieldDriver:       invalidating.Driver,
				invalidating.ConfigFieldAdaptor: "local1",
				invalidating.ConfigFieldBroker:  "main",
			},
		}, true)
		received := make(chan string, 10)
		sub, err := appBroker(t, app).Subscribe(context.Background(), "flam.cache.invalidations.main",
			func(_ context.Context, msg flam.IBrokerMessage) error {
				received <- string(msg.Payload())
				return nil
			})
		testutil.NoError(t, err)
		defer func() { _ = sub.Close() }()

		adaptors, err := invalidatingAdaptors(t, app, "main")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", adaptors[0].Id(), "main")
		testutil.NoError(t, adaptors[0].Set(context.Background(), &user{Id: 1}))
		testutil.Equal(t, "published", <-received, `["adaptors.user:1"]`)
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		app := newInvalidatingApp(t, map[string]any{
			cache.AdaptorConfigPath + ".main":        container.Bag{factory.ConfigFieldDriver: invalidating.Driver},
			invalidating.DefaultAdaptorConfigPath:    "local1",
			invalidating.DefaultBrokerConfigPath:     "main",
			invalidating.DefaultTopicConfigPath:      "custom",
			invalidating.DefaultPublishSetConfigPath: false,
		}, true)
		received := make(chan string, 10)
		sub, err := appBroker(t, app).Subscribe(context.Background(), "custom",
			func(_ context.Context, msg flam.IBrokerMessage) error {
				received <- string(msg.Payload())
				return nil
			})
		testutil.NoError(t, err)
		defer func() { _ = sub.Close() }()

		adaptors, err := invalidatingAdaptors(t, app, "main")
		testutil.NoError(t, err)
		testutil.NoError(t, adaptors[0].Set(context.Background(), &user{Id: 1}))
		testutil.NoError(t, adaptors[0].Del(context.Background(), &user{Id: 2}))
		testutil.Equal(t, "published", <-received, `["adaptors.user:2"]`)
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, c := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{}, invalidating.ConfigFieldAdaptor},
			{container.Bag{invalidating.ConfigFieldAdaptor: ""}, invalidating.ConfigFieldAdaptor},
			{container.Bag{invalidating.ConfigFieldAdaptor: 1}, invalidating.ConfigFieldAdaptor},
			{container.Bag{invalidating.ConfigFieldAdaptor: "main"}, invalidating.ConfigFieldAdaptor},
			{container.Bag{invalidating.ConfigFieldAdaptor: "local1"}, invalidating.ConfigFieldBroker},
			{container.Bag{invalidating.ConfigFieldAdaptor: "local1", invalidating.ConfigFieldBroker: ""}, invalidating.ConfigFieldBroker},
			{container.Bag{invalidating.ConfigFieldAdaptor: "local1", invalidating.ConfigFieldBroker: "main",
				invalidating.ConfigFieldTopic: ""}, invalidating.ConfigFieldTopic},
			{container.Bag{invalidating.ConfigFieldAdaptor: "local1", invalidating.ConfigFieldBroker: "main",
				invalidating.ConfigFieldTopic: 1}, invalidating.ConfigFieldTopic},
			{container.Bag{invalidating.ConfigFieldAdaptor: "local1", invalidating.ConfigFieldBroker: "main",
				invalidating.ConfigFieldPublishSet: "yes"}, invalidating.ConfigFieldPublishSet},
		} {
			_, err := invalidatingAdaptor(t, c.fields, nil, false)
			testutil.Error(t, errors.Unwrap(err), invalidating.ErrInvalidField,
				"unable to create invalidating cache adaptor: invalid invalidating cache adaptor field",
				map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldField: c.field})
		}
	})

	t.Run("should fail with an unknown adaptor or broker", func(t *testing.T) {
		for _, c := range []struct {
			fields   container.Bag
			sentinel error
			field    string
		}{
			{container.Bag{invalidating.ConfigFieldAdaptor: "nope", invalidating.ConfigFieldBroker: "main"},
				invalidating.ErrAdaptor, invalidating.ErrorFieldAdaptor},
			{container.Bag{invalidating.ConfigFieldAdaptor: "local1", invalidating.ConfigFieldBroker: "nope"},
				invalidating.ErrBroker, invalidating.ErrorFieldBroker},
		} {
			_, err := invalidatingAdaptor(t, c.fields, nil, true)
			if !errors.Is(err, c.sentinel) || !errors.Is(err, factory.ErrUnknownID) {
				t.Fatalf("Get() = %v, want %v and factory.ErrUnknownID", err, c.sentinel)
			}
			var e *flam.Error
			errors.As(errors.Unwrap(err), &e)
			testutil.Context(t, e, map[string]any{invalidating.ErrorFieldID: "main", c.field: "nope"})
		}
	})

	t.Run("should fail without the broker factory", func(t *testing.T) {
		_, err := invalidatingAdaptor(t, container.Bag{
			invalidating.ConfigFieldAdaptor: "local1",
			invalidating.ConfigFieldBroker:  "main",
		}, nil, false)
		testutil.Error(t, errors.Unwrap(err), invalidating.ErrBroker,
			"unable to create invalidating cache adaptor: unable to get invalidating cache adaptor broker: no broker factory",
			map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldBroker: "main"})
	})

	t.Run("should fail without the adaptor factory", func(t *testing.T) {
		// a factory of the creators other than the one of the cache bundle
		// hands itself to none
		type args struct {
			dig.In

			Creators []flam.ICacheAdaptorCreator `group:"flam.cache.adaptors.creator"`
		}
		app := newInvalidatingApp(t, nil, false)
		testutil.NoError(t, app.Invoke(func(a args) {
			f, err := factory.New(container.Bag{"main": container.Bag{
				factory.ConfigFieldDriver:       invalidating.Driver,
				invalidating.ConfigFieldAdaptor: "local1",
				invalidating.ConfigFieldBroker:  "main",
			}}, a.Creators...)
			testutil.NoError(t, err)
			_, err = f.Get("main")
			testutil.Error(t, errors.Unwrap(err), invalidating.ErrAdaptor,
				"unable to create invalidating cache adaptor: unable to get invalidating cache adaptor wrapped adaptor: no cache adaptor factory",
				map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldAdaptor: "local1"})
		}))
	})
}

func Test_InvalidatingNewAdaptor(t *testing.T) {
	local, _ := newMemoryAdaptor(t, 10, 0)
	for _, tc := range []struct {
		wrapped  flam.ICacheAdaptor
		broker   flam.IBroker
		topic    string
		sentinel error
		text     string
		ctx      map[string]any
	}{
		{nil, &fakeBroker{}, "topic", invalidating.ErrNilAdaptor, "nil invalidating cache adaptor wrapped adaptor",
			map[string]any{invalidating.ErrorFieldID: "main"}},
		{(*recorder)(nil), &fakeBroker{}, "topic", invalidating.ErrNilAdaptor, "nil invalidating cache adaptor wrapped adaptor",
			map[string]any{invalidating.ErrorFieldID: "main"}},
		{struct{ flam.ICacheAdaptor }{local}, &fakeBroker{}, "topic", invalidating.ErrNotKeyed,
			"invalidating cache adaptor wrapped adaptor tells no keys",
			map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldAdaptor: "main"}},
		{local, nil, "topic", invalidating.ErrNilBroker, "nil invalidating cache adaptor broker",
			map[string]any{invalidating.ErrorFieldID: "main"}},
		{local, (*fakeBroker)(nil), "topic", invalidating.ErrNilBroker, "nil invalidating cache adaptor broker",
			map[string]any{invalidating.ErrorFieldID: "main"}},
		{local, &fakeBroker{}, "", invalidating.ErrInvalidField, "invalid invalidating cache adaptor field",
			map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldField: invalidating.ConfigFieldTopic}},
		{local, &fakeBroker{subscribeErr: errBoom}, "topic", invalidating.ErrSubscribe,
			"unable to subscribe invalidating cache adaptor: boom",
			map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldTopic: "topic"}},
	} {
		a, err := invalidating.NewAdaptor("main", tc.wrapped, tc.broker, tc.topic, true)
		testutil.Error(t, err, tc.sentinel, "unable to create invalidating cache adaptor: "+tc.text, tc.ctx)
		testutil.Equal(t, "adaptor", a, nil)
	}
	if _, err := invalidating.NewAdaptor("main", local, &fakeBroker{subscribeErr: errBoom}, "topic", true); !errors.Is(err, errBoom) {
		t.Fatalf("NewAdaptor() = %v, want boom", err)
	}
}

func Test_InvalidatingAdaptor(t *testing.T) {
	t.Run("should read and write in the wrapped adaptor", func(t *testing.T) {
		a, local, b := newFakeInvalidating(t, true)
		testutil.Equal(t, "Id()", a.Id(), "main")
		testutil.Equal(t, "topic", b.topic, "topic")
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute, &user{Id: 2}))
		testutil.Equal(t, "Has() of the wrapped", has(t, local, 1, 2, 3), []bool{true, true, false})
		found, err := a.Has(context.Background(), &user{Id: 1}, &user{Id: 3})
		testutil.NoError(t, err)
		testutil.Equal(t, "Has()", found, []bool{true, false})
		got := &user{Id: 1}
		values, err := a.Get(context.Background(), got)
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", values, []any{got})
		testutil.Equal(t, "Name", got.Name, "ann")
		keys, err := a.Keys(&user{Id: 1})
		testutil.NoError(t, err)
		testutil.Equal(t, "Keys()", keys, []string{"adaptors.user:1"})

		testutil.NoError(t, a.Del(context.Background(), &user{Id: 1}))
		testutil.NoError(t, a.DelKeys(context.Background(), "adaptors.user:2"))
		testutil.Equal(t, "Has() after Del", has(t, local, 1, 2), []bool{false, false})
	})

	t.Run("should publish the keys removed and stored, with the instance", func(t *testing.T) {
		a, _, b := newFakeInvalidating(t, true)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute, &user{Id: 3}))
		var ctx context.Context // replaced with context.Background
		testutil.NoError(t, a.Del(ctx, &user{Id: 1}))
		testutil.NoError(t, a.DelKeys(context.Background(), "x", "y"))
		testutil.Equal(t, "payloads", b.payloads(), []string{
			`["adaptors.user:1","adaptors.user:2"]`, `["adaptors.user:3"]`, `["adaptors.user:1"]`, `["x","y"]`,
		})
		instance := b.published[0].Headers()[invalidating.HeaderInstance]
		if instance == "" {
			t.Fatal("no instance header")
		}
		for _, msg := range b.published {
			testutil.Equal(t, "topic", msg.Topic(), "topic")
			testutil.Equal(t, "headers", msg.Headers(), map[string]string{invalidating.HeaderInstance: instance})
		}

		other, _, otherBroker := newFakeInvalidating(t, true)
		testutil.NoError(t, other.Del(context.Background(), &user{Id: 1}))
		if otherBroker.published[0].Headers()[invalidating.HeaderInstance] == instance {
			t.Fatal("two adaptors share their instance")
		}
	})

	t.Run("should not publish the keys stored without publish set", func(t *testing.T) {
		a, local, b := newFakeInvalidating(t, false)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute, &user{Id: 2}))
		testutil.Equal(t, "Has()", has(t, local, 1, 2), []bool{true, true})
		testutil.NoError(t, a.Del(context.Background(), &user{Id: 1}))
		testutil.Equal(t, "payloads", b.payloads(), []string{`["adaptors.user:1"]`})
	})

	t.Run("should publish nothing for no objects", func(t *testing.T) {
		a, _, b := newFakeInvalidating(t, true)
		testutil.NoError(t, a.Set(context.Background()))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute))
		testutil.NoError(t, a.Del(context.Background()))
		testutil.NoError(t, a.DelKeys(context.Background()))
		testutil.Equal(t, "payloads", b.payloads(), []string{})
	})

	t.Run("should remove the keys of the other adaptors, ignoring its own", func(t *testing.T) {
		a, local, b := newFakeInvalidating(t, true)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}, &user{Id: 3}))
		own := b.published[0]
		testutil.NoError(t, b.handler(context.Background(), own))
		testutil.Equal(t, "Has() after its own", has(t, local, 1, 2, 3), []bool{true, true, true})

		other := broker.NewMessage("", "topic", []byte(`["adaptors.user:1","adaptors.user:3","missing"]`),
			map[string]string{invalidating.HeaderInstance: "other"})
		testutil.NoError(t, b.handler(context.Background(), other))
		testutil.NoError(t, b.handler(context.Background(), broker.NewMessage("", "topic", []byte(`[]`), nil)))
		testutil.Equal(t, "Has() after the other", has(t, local, 1, 2, 3), []bool{false, true, false})
		testutil.Equal(t, "payloads", len(b.payloads()), 1)
	})

	t.Run("should fail to handle a payload that is not a list of keys", func(t *testing.T) {
		_, _, b := newFakeInvalidating(t, true)
		err := b.handler(context.Background(), broker.NewMessage("", "topic", []byte(`{"a":1}`), nil))
		testutil.Error(t, err, invalidating.ErrDecode,
			"unable to handle invalidating cache keys: unable to decode invalidating cache adaptor keys: json: cannot unmarshal object into Go value of type []string",
			map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldTopic: "topic"})
	})

	t.Run("should fail to handle the keys the wrapped adaptor fails to remove", func(t *testing.T) {
		_, _, b := newFakeInvalidating(t, true)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := b.handler(ctx, broker.NewMessage("", "topic", []byte(`["k"]`), nil))
		if !errors.Is(err, cache.ErrStorage) {
			t.Fatalf("handler() = %v, want cache.ErrStorage", err)
		}
	})

	t.Run("should fail with the errors of the broker, after writing", func(t *testing.T) {
		a, local, b := newFakeInvalidating(t, true)
		b.publishErr = errBoom
		for _, c := range []struct {
			err     error
			message string
		}{
			{a.Set(context.Background(), &user{Id: 1}), "unable to set in invalidating cache"},
			{a.SetEphemeral(context.Background(), time.Minute, &user{Id: 2}), "unable to set in invalidating cache"},
			{a.Del(context.Background(), &user{Id: 3}), "unable to delete from invalidating cache"},
		} {
			testutil.Error(t, c.err, invalidating.ErrPublish,
				c.message+": unable to publish invalidating cache adaptor keys: boom",
				map[string]any{invalidating.ErrorFieldID: "main", invalidating.ErrorFieldTopic: "topic"})
		}
		testutil.Equal(t, "Has()", has(t, local, 1, 2), []bool{true, true})
	})

	t.Run("should fail with the errors of the wrapped adaptor, publishing nothing", func(t *testing.T) {
		g, s := memoryParts(t)
		local, err := memory.NewAdaptor("main", failingGenerator{g}, s, testutil.NewClock(memoryNow), 10, 0)
		testutil.NoError(t, err)
		b := &fakeBroker{}
		a, err := invalidating.NewAdaptor("main", local, b, "topic", true)
		testutil.NoError(t, err)
		bad := &user{Name: "bad"}
		testutil.Error(t, a.Set(context.Background(), &user{Id: 1}, bad), errBoom, "unable to get memory cache keys: boom",
			map[string]any{memory.ErrorFieldID: "main"})
		testutil.Error(t, a.Del(context.Background(), bad), errBoom, "unable to get memory cache keys: boom",
			map[string]any{memory.ErrorFieldID: "main"})
		testutil.Equal(t, "Has()", has(t, local, 1), []bool{false})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, err := range []error{
			a.Set(ctx, &user{Id: 1}),
			a.SetEphemeral(ctx, time.Minute, &user{Id: 1}),
			a.Del(ctx, &user{Id: 1}),
		} {
			if !errors.Is(err, cache.ErrStorage) {
				t.Fatalf("error = %v, want cache.ErrStorage", err)
			}
		}
		testutil.Equal(t, "payloads", b.payloads(), []string{})
	})

	t.Run("should close the subscription once", func(t *testing.T) {
		a, _, b := newFakeInvalidating(t, true)
		b.sub.err = errBoom
		closer := a.(interface{ Close() error })
		for range 2 {
			if err := closer.Close(); err != errBoom {
				t.Fatalf("Close() = %v, want boom", err)
			}
		}
		testutil.Equal(t, "closes", b.sub.closes, 1)
	})
}

func Test_InvalidatingPropagation(t *testing.T) {
	t.Run("should remove the values of the other instances through the broker", func(t *testing.T) {
		app := newInvalidatingApp(t, map[string]any{
			cache.AdaptorConfigPath + ".a": container.Bag{
				factory.ConfigFieldDriver:       invalidating.Driver,
				invalidating.ConfigFieldAdaptor: "local1",
				invalidating.ConfigFieldBroker:  "main",
				invalidating.ConfigFieldTopic:   "users",
			},
			cache.AdaptorConfigPath + ".b": container.Bag{
				factory.ConfigFieldDriver:       invalidating.Driver,
				invalidating.ConfigFieldAdaptor: "local2",
				invalidating.ConfigFieldBroker:  "main",
				invalidating.ConfigFieldTopic:   "users",
			},
		}, true)
		adaptors, err := invalidatingAdaptors(t, app, "a", "b", "local1", "local2")
		testutil.NoError(t, err)
		a, b, local1, local2 := adaptors[0], adaptors[1], adaptors[2], adaptors[3]
		for _, local := range []flam.ICacheAdaptor{local1, local2} {
			testutil.NoError(t, local.Set(context.Background(), &user{Id: 1}, &user{Id: 2}, &user{Id: 3}))
		}

		testutil.NoError(t, a.Del(context.Background(), &user{Id: 1}))
		testutil.Equal(t, "local1 after Del", has(t, local1, 1), []bool{false})
		eventually(t, "local2 removes 1", func() bool { return !has(t, local2, 1)[0] })

		testutil.NoError(t, b.Set(context.Background(), &user{Id: 2, Name: "new"}))
		eventually(t, "local1 removes 2", func() bool { return !has(t, local1, 2)[0] })
		testutil.Equal(t, "local2 keeps its own", has(t, local2, 2), []bool{true})
		testutil.Equal(t, "local1 keeps 3", has(t, local1, 3), []bool{true})
		testutil.Equal(t, "local2 keeps 3", has(t, local2, 3), []bool{true})
	})

	t.Run("should ignore its own messages", func(t *testing.T) {
		app := newInvalidatingApp(t, nil, true)
		b := appBroker(t, app)
		local1, _ := newMemoryAdaptor(t, 10, 0)
		local2, _ := newMemoryAdaptor(t, 10, 0)
		rec1 := &recorder{ICacheKeyedAdaptor: local1.(flam.ICacheKeyedAdaptor)}
		rec2 := &recorder{ICacheKeyedAdaptor: local2.(flam.ICacheKeyedAdaptor)}
		first, err := invalidating.NewAdaptor("first", rec1, b, "users", true)
		testutil.NoError(t, err)
		second, err := invalidating.NewAdaptor("second", rec2, b, "users", true)
		testutil.NoError(t, err)
		defer func() {
			testutil.NoError(t, first.(interface{ Close() error }).Close())
			testutil.NoError(t, second.(interface{ Close() error }).Close())
		}()

		// the messages of a topic reach a subscription in order: once the
		// first adaptor removed the key of the second, it handled its own
		testutil.NoError(t, first.DelKeys(context.Background(), "x"))
		testutil.NoError(t, second.DelKeys(context.Background(), "y"))
		eventually(t, "first removes y", func() bool { return len(rec1.calls()) == 2 })
		eventually(t, "second removes x", func() bool { return len(rec2.calls()) == 2 })
		testutil.Equal(t, "first", rec1.calls(), [][]string{{"x"}, {"y"}})
		// the second removed y, and x, in any order, as its own removal
		// races with the handling of the message of the first
		calls := rec2.calls()
		if calls[0][0] == "x" {
			calls[0], calls[1] = calls[1], calls[0]
		}
		testutil.Equal(t, "second", calls, [][]string{{"y"}, {"x"}})
	})
}
