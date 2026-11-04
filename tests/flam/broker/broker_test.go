package broker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// fakeSubscription is a flam.IBrokerSubscription ending when closed, or
// when its end method is called with the error ending it.
type fakeSubscription struct {
	topic    string
	closeErr error

	once   sync.Once
	done   chan struct{}
	mu     sync.Mutex
	err    error
	closes int
}

func newFakeSubscription(topic string) *fakeSubscription {
	return &fakeSubscription{topic: topic, done: make(chan struct{})}
}

func (s *fakeSubscription) Topic() string { return s.topic }

func (s *fakeSubscription) Done() <-chan struct{} { return s.done }

func (s *fakeSubscription) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *fakeSubscription) Close() error {
	s.mu.Lock()
	s.closes++
	s.mu.Unlock()
	s.end(nil)
	return s.closeErr
}

func (s *fakeSubscription) end(err error) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.done)
	})
}

func (s *fakeSubscription) closed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closes
}

// published is a message given to the Publish method of a fakeBroker.
type published struct {
	topic   string
	payload string
	headers map[string]string
}

// fakeBroker is a flam.IBroker recording the publications, handing the
// handler of its last subscription to the test, and returning the
// subscription and errors set by the test.
type fakeBroker struct {
	id           string
	publishErr   error
	subscribeErr error
	sub          *fakeSubscription

	mu        sync.Mutex
	published []published
	handler   flam.BrokerHandler
	ctx       context.Context
	closed    bool
}

func (b *fakeBroker) Id() string { return b.id }

func (b *fakeBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *fakeBroker) Publish(_ context.Context, topic string, payload []byte, headers map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.published = append(b.published, published{topic: topic, payload: string(payload), headers: headers})
	return b.publishErr
}

func (b *fakeBroker) Subscribe(ctx context.Context, topic string, handler flam.BrokerHandler) (flam.IBrokerSubscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subscribeErr != nil {
		return nil, b.subscribeErr
	}
	b.handler = handler
	b.ctx = ctx
	if b.sub == nil {
		b.sub = newFakeSubscription(topic)
	}
	return b.sub, nil
}

func (b *fakeBroker) lastHandler() flam.BrokerHandler {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.handler
}

func (b *fakeBroker) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// fakeCreator is a flam.IBrokerCreator of the "fake" driver, creating
// fake brokers.
type fakeCreator struct {
	mu      sync.Mutex
	created []*fakeBroker
}

func (c *fakeCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (c *fakeCreator) Create(bag flam.IBag) (flam.IBroker, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := &fakeBroker{id: bag.String(factory.ConfigFieldId)}
	c.created = append(c.created, b)
	return b, nil
}

// newApp returns a testutil.App with the config and broker bundles, the
// fake creator and the broker "main" of the fake driver.
func newApp(t *testing.T, creator *fakeCreator) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(broker.NewBundle()))
	testutil.NoError(t, a.Provide(func() flam.IBrokerCreator { return creator }, dig.Group(broker.BrokerCreatorGroup)))
	bag := container.Bag{}
	testutil.NoError(t, bag.Set(broker.BrokerConfigPath+".main", container.Bag{factory.ConfigFieldDriver: "fake"}))
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "BrokerConfigPath", broker.BrokerConfigPath, "flam.broker.brokers")
	testutil.Equal(t, "BrokerCreatorGroup", broker.BrokerCreatorGroup, "flam.broker.brokers.creator")
	testutil.Equal(t, "ErrorFieldID", broker.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldTopic", broker.ErrorFieldTopic, "topic")
	testutil.Equal(t, "ErrorFieldMessage", broker.ErrorFieldMessage, "message")
	testutil.Equal(t, "ErrorFieldPanic", broker.ErrorFieldPanic, "panic")
	testutil.Equal(t, "ErrorFieldStack", broker.ErrorFieldStack, "stack")
	for sentinel, text := range map[error]string{
		broker.ErrClosed:       "broker closed",
		broker.ErrInvalidTopic: "invalid broker topic",
		broker.ErrNilHandler:   "nil broker handler",
		broker.ErrNilBroker:    "nil broker",
		broker.ErrPublish:      "unable to publish broker message",
		broker.ErrSubscribe:    "unable to subscribe to broker topic",
		broker.ErrReceive:      "unable to receive broker message",
		broker.ErrHandlerPanic: "broker handler panicked",
		broker.ErrEncode:       "unable to encode broker message",
		broker.ErrDecode:       "unable to decode broker message",
		broker.ErrRunning:      "broker process running",
	} {
		testutil.Equal(t, "sentinel", sentinel.Error(), text)
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide a single broker factory of the configuration and creators", func(t *testing.T) {
		b := broker.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.broker")
		if b == broker.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}

		creator := &fakeCreator{}
		a := newApp(t, creator)
		var first, second flam.IBrokerFactory
		testutil.NoError(t, a.Invoke(func(f flam.IBrokerFactory) { first = f }))
		testutil.NoError(t, a.Invoke(func(f flam.IBrokerFactory) { second = f }))
		if first == nil || first != second {
			t.Fatal("the container did not provide a single IBrokerFactory")
		}
		testutil.Equal(t, "Available()", first.Available(), []string{"main"})
		got, err := first.Get("main")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", got.Id(), "main")

		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
		if !creator.created[0].isClosed() {
			t.Error("closing the bundle did not close the broker")
		}
		if _, err := first.Get("main"); !errors.Is(err, factory.ErrClosed) {
			t.Errorf("Get() = %v, want factory.ErrClosed", err)
		}
	})

	t.Run("should register its provider", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, broker.NewBundle().Register(a))
		testutil.Equal(t, "calls", a.Calls(), []string{"provider/0"})
	})

	t.Run("should fail to close when the factory cannot be created", func(t *testing.T) {
		a := testutil.NewApp()
		b := broker.NewBundle()
		testutil.NoError(t, b.Register(a))
		if err := b.(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail without configuration")
		}
	})
}

func Test_NewMessage(t *testing.T) {
	headers := map[string]string{"a": "1"}
	m := broker.NewMessage("id", "topic", []byte("payload"), headers)
	headers["a"] = "changed"
	testutil.Equal(t, "Id()", m.Id(), "id")
	testutil.Equal(t, "Topic()", m.Topic(), "topic")
	testutil.Equal(t, "Payload()", m.Payload(), []byte("payload"))
	testutil.Equal(t, "Headers()", m.Headers(), map[string]string{"a": "1"})

	empty := broker.NewMessage("", "topic", nil, nil)
	testutil.Equal(t, "Headers()", empty.Headers(), map[string]string{})
}

// order is the value of the JSON tests.
type order struct {
	Id    int    `json:"id"`
	Label string `json:"label"`
}

func Test_PublishJSON(t *testing.T) {
	ctx := context.Background()

	t.Run("should publish the JSON encoding of the value", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		testutil.NoError(t, broker.PublishJSON(ctx, b, "orders", order{Id: 1, Label: "a"}, map[string]string{"h": "v"}))
		testutil.Equal(t, "published", b.published, []published{
			{topic: "orders", payload: `{"id":1,"label":"a"}`, headers: map[string]string{"h": "v"}},
		})
	})

	t.Run("should return the error of the broker", func(t *testing.T) {
		b := &fakeBroker{id: "main", publishErr: errors.New("boom")}
		testutil.Equal(t, "error", broker.PublishJSON(ctx, b, "orders", 1, nil), b.publishErr)
	})

	t.Run("should fail with a nil broker", func(t *testing.T) {
		testutil.Error(t, broker.PublishJSON(ctx, nil, "orders", 1, nil), broker.ErrNilBroker,
			"unable to publish broker json message: nil broker", map[string]any{"topic": "orders"})
	})

	t.Run("should fail with a value that cannot be encoded", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		err := broker.PublishJSON(ctx, b, "orders", make(chan int), nil)
		testutil.Error(t, err, broker.ErrEncode,
			"unable to publish broker json message: unable to encode broker message: json: unsupported type: chan int",
			map[string]any{"topic": "orders"})
		testutil.Equal(t, "published", len(b.published), 0)
	})
}

func Test_SubscribeJSON(t *testing.T) {
	ctx := context.Background()

	t.Run("should hand the decoded values to the handler", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		var got []order
		sub, err := broker.SubscribeJSON(ctx, b, "orders", func(_ context.Context, msg flam.IBrokerMessage, value order) error {
			testutil.Equal(t, "Topic()", msg.Topic(), "orders")
			got = append(got, value)
			return errors.New("handled")
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "subscription", sub, flam.IBrokerSubscription(b.sub))

		err = b.lastHandler()(ctx, broker.NewMessage("1", "orders", []byte(`{"id":7,"label":"x"}`), nil))
		testutil.Equal(t, "error", err.Error(), "handled")
		testutil.Equal(t, "values", got, []order{{Id: 7, Label: "x"}})
	})

	t.Run("should fail the messages that cannot be decoded", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		called := false
		_, err := broker.SubscribeJSON(ctx, b, "orders", func(context.Context, flam.IBrokerMessage, order) error {
			called = true
			return nil
		})
		testutil.NoError(t, err)

		err = b.lastHandler()(ctx, broker.NewMessage("42", "orders", []byte("{"), nil))
		testutil.Error(t, err, broker.ErrDecode,
			"unable to subscribe to broker json messages: unable to decode broker message: unexpected end of JSON input",
			map[string]any{"topic": "orders", "message": "42"})
		testutil.Equal(t, "called", called, false)
	})

	t.Run("should return the error of the broker", func(t *testing.T) {
		b := &fakeBroker{id: "main", subscribeErr: errors.New("boom")}
		_, err := broker.SubscribeJSON(ctx, b, "orders", func(context.Context, flam.IBrokerMessage, order) error { return nil })
		testutil.Equal(t, "error", err, b.subscribeErr)
	})

	t.Run("should fail with a nil broker or handler", func(t *testing.T) {
		_, err := broker.SubscribeJSON(ctx, nil, "orders", func(context.Context, flam.IBrokerMessage, order) error { return nil })
		testutil.Error(t, err, broker.ErrNilBroker,
			"unable to subscribe to broker json messages: nil broker", map[string]any{"topic": "orders"})
		_, err = broker.SubscribeJSON[order](ctx, &fakeBroker{}, "orders", nil)
		testutil.Error(t, err, broker.ErrNilHandler,
			"unable to subscribe to broker json messages: nil broker handler", map[string]any{"topic": "orders"})
	})
}

// handler is a flam.BrokerHandler doing nothing.
func handler(context.Context, flam.IBrokerMessage) error { return nil }

// runProcess runs the process in a goroutine, returning the channel of
// the error of its Run, once the process runs.
func runProcess(t *testing.T, ctx context.Context, p flam.IWatchdogProcess, b *fakeBroker) <-chan error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- p.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for b.lastHandler() == nil || !p.IsRunning() {
		if time.Now().After(deadline) {
			t.Fatal("the process did not subscribe")
		}
		time.Sleep(time.Millisecond)
	}
	return result
}

func Test_NewProcess(t *testing.T) {
	t.Run("should fail with a nil broker or handler", func(t *testing.T) {
		_, err := broker.NewProcess("p", nil, "orders", handler)
		testutil.Error(t, err, broker.ErrNilBroker,
			"unable to create broker process: nil broker", map[string]any{"id": "p", "topic": "orders"})
		_, err = broker.NewProcess("p", &fakeBroker{}, "orders", nil)
		testutil.Error(t, err, broker.ErrNilHandler,
			"unable to create broker process: nil broker handler", map[string]any{"id": "p", "topic": "orders"})
	})

	t.Run("should subscribe until the context is done", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		p, err := broker.NewProcess("p", b, "orders", handler)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", p.Id(), "p")
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)

		ctx, cancel := context.WithCancel(context.Background())
		result := runProcess(t, ctx, p, b)
		testutil.Error(t, p.Run(ctx), broker.ErrRunning,
			"unable to run broker process: broker process running", map[string]any{"id": "p", "topic": "orders"})
		cancel()
		testutil.NoError(t, <-result)
		testutil.Equal(t, "closes", b.sub.closed(), 1)
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		testutil.NoError(t, p.Close())
		if b.isClosed() {
			t.Error("closing the process closed the broker")
		}
	})

	t.Run("should subscribe until closed", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		b.sub = newFakeSubscription("orders")
		b.sub.closeErr = errors.New("close")
		p, _ := broker.NewProcess("p", b, "orders", handler)
		// a nil context never ends
		result := runProcess(t, nil, p, b)
		testutil.NoError(t, p.Close())
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "error", (<-result).Error(), "close")
		testutil.NoError(t, p.Run(context.Background()))
	})

	t.Run("should end with its subscription", func(t *testing.T) {
		b := &fakeBroker{id: "main"}
		p, _ := broker.NewProcess("p", b, "orders", handler)
		result := runProcess(t, context.Background(), p, b)
		ended := errors.New("ended")
		b.sub.end(ended)
		if err := <-result; !errors.Is(err, ended) {
			t.Fatalf("Run() = %v, want %v", err, ended)
		}
	})

	t.Run("should return the error of the broker", func(t *testing.T) {
		b := &fakeBroker{id: "main", subscribeErr: errors.New("boom")}
		p, _ := broker.NewProcess("p", b, "orders", handler)
		testutil.Equal(t, "error", p.Run(context.Background()), b.subscribeErr)
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
	})
}
