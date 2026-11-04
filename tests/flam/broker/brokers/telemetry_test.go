package brokers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	brokerkafka "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/kafka"
	brokernats "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/nats"
	brokerredis "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// telemetry is what an app provides to trace and measure its brokers,
// each part being provided when not nil.
type telemetry struct {
	recorder   *tracetest.SpanRecorder
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
	registry   *prometheus.Registry
}

// newTelemetry returns a telemetry recording the spans of its provider,
// propagating the W3C trace context, and registering on a registry of its
// own.
func newTelemetry() *telemetry {
	recorder := tracetest.NewSpanRecorder()
	return &telemetry{
		recorder:   recorder,
		provider:   sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
		propagator: propagation.TraceContext{},
		registry:   prometheus.NewRegistry(),
	}
}

// setup provides the parts of the telemetry that are not nil to the app.
func (tel *telemetry) setup(a *testutil.App) {
	if tel.provider != nil {
		_ = a.Provide(func() trace.TracerProvider { return tel.provider })
	}
	if tel.propagator != nil {
		_ = a.Provide(func() propagation.TextMapPropagator { return tel.propagator })
	}
	if tel.registry != nil {
		_ = a.Provide(func() prometheus.Registerer { return tel.registry })
	}
}

// spans waits until n spans ended in the recorder, and returns them.
func (tel *telemetry) spans(t *testing.T, n int) []sdktrace.ReadOnlySpan {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		spans := tel.recorder.Ended()
		if len(spans) >= n {
			return spans
		}
		if time.Now().After(deadline) {
			t.Fatalf("spans = %d, want %d", len(spans), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// named returns the spans of the name.
func named(spans []sdktrace.ReadOnlySpan, name string) []sdktrace.ReadOnlySpan {
	var found []sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == name {
			found = append(found, span)
		}
	}
	return found
}

// metrics asserts that the metrics of the names gathered from the registry
// are the ones of the text exposition format.
func (tel *telemetry) metrics(t *testing.T, want string, names ...string) {
	t.Helper()
	if err := promtestutil.GatherAndCompare(tel.registry, strings.NewReader(want), names...); err != nil {
		t.Error(err)
	}
}

// attrs returns the attributes of the span, by key.
func attrs(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	all := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		all[kv.Key] = kv.Value
	}
	return all
}

// traced is a message handed to a tracing handler, with the span context
// of the context given to the handler.
type traced struct {
	msg  flam.IBrokerMessage
	span trace.SpanContext
}

// tracing returns a handler sending the messages it receives, with the
// span context of its context, to the channel returned along with it,
// then returning the error of fail for the number of the message, from 1,
// when fail is not nil.
func tracing(fail func(n int) error) (flam.BrokerHandler, <-chan traced) {
	ch := make(chan traced, 100)
	n := 0
	return func(ctx context.Context, msg flam.IBrokerMessage) error {
		n++
		ch <- traced{msg: msg, span: trace.SpanContextFromContext(ctx)}
		if fail != nil {
			return fail(n)
		}
		return nil
	}, ch
}

// nextTraced returns the next message of the channel, failing the test
// when none comes in time.
func nextTraced(t *testing.T, ch <-chan traced) traced {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		t.Fatal("no message received")
		return traced{}
	}
}

// assertPropagated publishes a message on the topic of the broker within a
// parent span, the handler of the subscription to the topic, of the
// broker, receiving it, and asserts that the producer span is the child
// of the parent, holding the attributes of the system, that the consumer
// span is the child of the producer span, holding the message id unless
// empty, and that the handler ran within it. It returns the producer and
// the consumer spans.
func assertPropagated(t *testing.T, tel *telemetry, b flam.IBroker, topic, subscribed, system string) (sdktrace.ReadOnlySpan, sdktrace.ReadOnlySpan) {
	t.Helper()
	handler, ch := tracing(nil)
	sub, err := b.Subscribe(context.Background(), subscribed, handler)
	testutil.NoError(t, err)
	defer func() { _ = sub.Close() }()

	ctx, parent := tel.provider.Tracer("test").Start(context.Background(), "parent")
	headers := map[string]string{"h": "v"}
	testutil.NoError(t, b.Publish(ctx, topic, []byte("payload"), headers))
	parent.End()
	testutil.Equal(t, "caller headers", headers, map[string]string{"h": "v"})

	got := nextTraced(t, ch)
	testutil.Equal(t, "header", got.msg.Headers()["h"], "v")
	spans := tel.spans(t, 3)
	producer := named(spans, "publish "+topic)[0]
	consumer := named(spans, "process "+subscribed)[0]
	testutil.Equal(t, "traceparent", got.msg.Headers()["traceparent"],
		"00-"+producer.SpanContext().TraceID().String()+"-"+producer.SpanContext().SpanID().String()+"-01")

	testutil.Equal(t, "producer kind", producer.SpanKind(), trace.SpanKindProducer)
	testutil.Equal(t, "producer parent", producer.Parent().SpanID(), parent.SpanContext().SpanID())
	testutil.Equal(t, "producer scope", producer.InstrumentationScope().Name, broker.ScopeName)
	testutil.Equal(t, "producer status", producer.Status().Code, codes.Unset)
	pa := attrs(producer)
	testutil.Equal(t, "producer system", pa["messaging.system"].AsString(), system)
	testutil.Equal(t, "producer destination", pa["messaging.destination.name"].AsString(), topic)
	testutil.Equal(t, "producer operation type", pa["messaging.operation.type"].AsString(), "send")
	testutil.Equal(t, "producer operation", pa["messaging.operation.name"].AsString(), "publish")

	testutil.Equal(t, "consumer kind", consumer.SpanKind(), trace.SpanKindConsumer)
	testutil.Equal(t, "consumer trace", consumer.SpanContext().TraceID(), parent.SpanContext().TraceID())
	testutil.Equal(t, "consumer parent", consumer.Parent().SpanID(), producer.SpanContext().SpanID())
	testutil.Equal(t, "handler span", got.span, consumer.SpanContext())
	testutil.Equal(t, "consumer status", consumer.Status().Code, codes.Unset)
	ca := attrs(consumer)
	testutil.Equal(t, "consumer system", ca["messaging.system"].AsString(), system)
	testutil.Equal(t, "consumer destination", ca["messaging.destination.name"].AsString(), topic)
	testutil.Equal(t, "consumer operation type", ca["messaging.operation.type"].AsString(), "process")
	testutil.Equal(t, "consumer operation", ca["messaging.operation.name"].AsString(), "process")
	testutil.Equal(t, "consumer size", ca["messaging.message.body.size"].AsInt64(), int64(len("payload")))
	if got.msg.Id() == "" {
		testutil.Equal(t, "consumer message id", ca["messaging.message.id"].Type(), attribute.EMPTY)
	} else {
		testutil.Equal(t, "consumer message id", ca["messaging.message.id"].AsString(), got.msg.Id())
	}
	if topic == subscribed {
		testutil.Equal(t, "consumer template", ca["messaging.destination.template"].Type(), attribute.EMPTY)
	} else {
		testutil.Equal(t, "consumer template", ca["messaging.destination.template"].AsString(), subscribed)
	}
	return producer, consumer
}

// metricsHeader returns the header of the exposition of the counter of
// the name and help, in the flam namespace.
func metricsHeader(name, help string) string {
	return "# HELP flam_broker_" + name + " " + help + "\n# TYPE flam_broker_" + name + " counter\n"
}

// Headers of the exposition of the counters of the brokers.
var (
	publishedHeader = metricsHeader("messages_published_total", "Number of messages published by the brokers, by broker, topic and result.")
	processedHeader = metricsHeader("messages_processed_total", "Number of messages handled by the subscriptions of the brokers, by broker, topic and result.")
)

func Test_Telemetry_Redis(t *testing.T) {
	for _, mode := range []string{brokerredis.ModePubSub, brokerredis.ModeStreams} {
		t.Run("should propagate the trace from the publisher to the handler in the "+mode+" mode", func(t *testing.T) {
			tel := newTelemetry()
			b := mustCreate(t, brokerredis.Driver, container.Bag{brokerredis.ConfigFieldMode: mode}, nil, tel.setup)
			assertPropagated(t, tel, b, "orders", "orders", "redis")
			tel.metrics(t, publishedHeader+`flam_broker_messages_published_total{broker="main",result="success",topic="orders"} 1
`+processedHeader+`flam_broker_messages_processed_total{broker="main",result="success",topic="orders"} 1
`, "flam_broker_messages_published_total", "flam_broker_messages_processed_total")
		})
	}

}

func Test_Telemetry_Nats(t *testing.T) {
	t.Run("should propagate the trace from the publisher to the handler with core NATS", func(t *testing.T) {
		tel := newTelemetry()
		s := natsServer(t, nil)
		b := mustCreate(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: s.ClientURL()}, nil, tel.setup)
		assertPropagated(t, tel, b, "orders.created", "orders.*", "nats")
		tel.metrics(t, processedHeader+`flam_broker_messages_processed_total{broker="main",result="success",topic="orders.*"} 1
`, "flam_broker_messages_processed_total")
	})

	t.Run("should propagate the trace from the publisher to the handler with JetStream", func(t *testing.T) {
		tel := newTelemetry()
		s := natsServer(t, nil)
		b := mustCreate(t, brokernats.Driver, container.Bag{
			brokernats.ConfigFieldURL:       s.ClientURL(),
			brokernats.ConfigFieldJetStream: true,
			brokernats.ConfigFieldStream:    "ORDERS",
			brokernats.ConfigFieldSubjects:  []any{"orders.>"},
		}, nil, tel.setup)
		assertPropagated(t, tel, b, "orders.created", "orders.created", "nats")
	})

	t.Run("should trace a publication of a nil context", func(t *testing.T) {
		tel := newTelemetry()
		s := natsServer(t, nil)
		b := mustCreate(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: s.ClientURL()}, nil, tel.setup)
		//lint:ignore SA1012 a nil context, never done, is what is tested
		testutil.NoError(t, b.Publish(nil, "orders", nil, nil)) //nolint:staticcheck // as above
		span := tel.spans(t, 1)[0]
		testutil.Equal(t, "Name()", span.Name(), "publish orders")
		testutil.Equal(t, "Parent()", span.Parent().IsValid(), false)
	})
}

func Test_Telemetry_Kafka(t *testing.T) {
	// deliver hands the messages written by the writer of the fake to its
	// first reader.
	deliver := func(t *testing.T, k *fakeKafka) {
		t.Helper()
		k.writer.mu.Lock()
		written := append([]kafkago.Message(nil), k.writer.messages...)
		k.writer.messages = nil
		k.writer.mu.Unlock()
		for _, m := range written {
			k.reader(0).messages <- m
		}
	}

	t.Run("should propagate the trace from the publisher to the handler", func(t *testing.T) {
		tel := newTelemetry()
		k := newFakeKafka()
		b := mustCreate(t, brokerkafka.Driver, container.Bag{}, nil, tel.setup, func(a *testutil.App) {
			testutil.NoError(t, a.Provide(func() flam.IBrokerKafkaClient { return k }))
		})
		k.onReader = func(*fakeReader) {
			go func() {
				for {
					k.writer.mu.Lock()
					n := len(k.writer.messages)
					k.writer.mu.Unlock()
					if n > 0 {
						deliver(t, k)
						return
					}
					time.Sleep(time.Millisecond)
				}
			}()
		}
		assertPropagated(t, tel, b, "orders", "orders", "kafka")
	})

	t.Run("should record the errors and the panics of the handler", func(t *testing.T) {
		tel := newTelemetry()
		k := newFakeKafka()
		b := mustCreate(t, brokerkafka.Driver, container.Bag{brokerkafka.ConfigFieldRetryBackoff: "1ms"}, nil, tel.setup, func(a *testutil.App) {
			testutil.NoError(t, a.Provide(func() flam.IBrokerKafkaClient { return k }))
		})
		handler, ch := tracing(func(n int) error {
			switch n {
			case 1:
				return errBoom
			case 2:
				panic("handler panic")
			}
			return nil
		})
		sub, err := b.Subscribe(context.Background(), "orders", handler)
		testutil.NoError(t, err)
		defer func() { _ = sub.Close() }()
		k.reader(0).messages <- kafkago.Message{Topic: "orders", Partition: 1, Offset: 7, Value: []byte("x")}
		for range 3 {
			nextTraced(t, ch)
		}
		spans := named(tel.spans(t, 3), "process orders")
		testutil.Equal(t, "spans", len(spans), 3)
		for i, want := range []string{broker.ResultError, broker.ResultPanic, ""} {
			a := attrs(spans[i])
			testutil.Equal(t, "message id", a["messaging.message.id"].AsString(), "1:7")
			testutil.Equal(t, "parent", spans[i].Parent().IsValid(), false)
			if want == "" {
				testutil.Equal(t, "status", spans[i].Status().Code, codes.Unset)
				continue
			}
			testutil.Equal(t, "error type", a["error.type"].AsString(), want)
			testutil.Equal(t, "status", spans[i].Status().Code, codes.Error)
			testutil.Equal(t, "events", spans[i].Events()[0].Name, "exception")
		}
		testutil.Equal(t, "description", spans[0].Status().Description, errBoom.Error())
		tel.metrics(t, processedHeader+`flam_broker_messages_processed_total{broker="main",result="error",topic="orders"} 1
flam_broker_messages_processed_total{broker="main",result="panic",topic="orders"} 1
flam_broker_messages_processed_total{broker="main",result="success",topic="orders"} 1
`, "flam_broker_messages_processed_total")
		count, err := promtestutil.GatherAndCount(tel.registry, "flam_broker_message_process_duration_seconds")
		testutil.NoError(t, err)
		testutil.Equal(t, "duration series", count, 3)
	})

	t.Run("should record the failure to publish", func(t *testing.T) {
		tel := newTelemetry()
		k := newFakeKafka()
		k.writer.err = errBoom
		b := mustCreate(t, brokerkafka.Driver, container.Bag{}, nil, tel.setup, func(a *testutil.App) {
			testutil.NoError(t, a.Provide(func() flam.IBrokerKafkaClient { return k }))
		})
		err := b.Publish(context.Background(), "orders", nil, map[string]string{brokerkafka.HeaderKey: "k"})
		if !errors.Is(err, broker.ErrPublish) {
			t.Fatalf("Publish() = %v, want ErrPublish", err)
		}
		span := tel.spans(t, 1)[0]
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: err.Error()})
		testutil.Equal(t, "error type", attrs(span)["error.type"].AsString(), broker.ResultError)
		testutil.Equal(t, "key", string(k.writer.messages[0].Key), "k")
		tel.metrics(t, publishedHeader+`flam_broker_messages_published_total{broker="main",result="error",topic="orders"} 1
`, "flam_broker_messages_published_total")
	})
}

func Test_Telemetry(t *testing.T) {
	ctx := context.Background()

	t.Run("should neither trace nor measure when disabled", func(t *testing.T) {
		tel := newTelemetry()
		s := natsServer(t, nil)
		b := mustCreate(t, brokernats.Driver, container.Bag{
			brokernats.ConfigFieldURL: s.ClientURL(),
			broker.ConfigFieldTracing: false,
			broker.ConfigFieldMetrics: false,
		}, nil, tel.setup)
		handler, ch := tracing(nil)
		_, err := b.Subscribe(ctx, "orders", handler)
		testutil.NoError(t, err)
		testutil.NoError(t, b.Publish(ctx, "orders", nil, nil))
		got := nextTraced(t, ch)
		testutil.Equal(t, "headers", got.msg.Headers(), map[string]string{})
		testutil.Equal(t, "span", got.span.IsValid(), false)
		testutil.Equal(t, "spans", len(tel.recorder.Ended()), 0)
		families, err := tel.registry.Gather()
		testutil.NoError(t, err)
		testutil.Equal(t, "families", len(families), 0)
	})

	t.Run("should read the defaults of the configuration", func(t *testing.T) {
		tel := newTelemetry()
		s := natsServer(t, nil)
		b := mustCreate(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: s.ClientURL()}, map[string]any{
			broker.DefaultTracingConfigPath:    false,
			broker.DefaultNamespaceConfigPath:  "app",
			broker.DefaultTopicLabelConfigPath: false,
		}, tel.setup)
		testutil.NoError(t, b.Publish(ctx, "orders", nil, nil))
		testutil.Equal(t, "spans", len(tel.recorder.Ended()), 0)
		tel.metrics(t, `# HELP app_broker_messages_published_total Number of messages published by the brokers, by broker, topic and result.
# TYPE app_broker_messages_published_total counter
app_broker_messages_published_total{broker="main",result="success",topic=""} 1
`, "app_broker_messages_published_total")
	})

	t.Run("should not trace without a propagator or a tracer provider", func(t *testing.T) {
		for _, missing := range []string{"propagator", "provider"} {
			tel := newTelemetry()
			if missing == "propagator" {
				tel.propagator = nil
			} else {
				tel.provider = nil
			}
			tel.registry = nil
			s := natsServer(t, nil)
			b := mustCreate(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: s.ClientURL()}, nil, tel.setup)
			handler, ch := tracing(nil)
			_, err := b.Subscribe(ctx, "orders", handler)
			testutil.NoError(t, err)
			testutil.NoError(t, b.Publish(ctx, "orders", nil, nil))
			testutil.Equal(t, missing+" headers", nextTraced(t, ch).msg.Headers(), map[string]string{})
			testutil.Equal(t, missing+" spans", len(tel.recorder.Ended()), 0)
		}
	})

	t.Run("should share the metrics of the brokers of a namespace", func(t *testing.T) {
		tel := newTelemetry()
		s := natsServer(t, nil)
		for range 2 {
			b := mustCreate(t, brokernats.Driver, container.Bag{brokernats.ConfigFieldURL: s.ClientURL()}, nil, tel.setup)
			testutil.NoError(t, b.Publish(ctx, "orders", nil, nil))
		}
		tel.metrics(t, publishedHeader+`flam_broker_messages_published_total{broker="main",result="success",topic="orders"} 2
`, "flam_broker_messages_published_total")
	})

	for driver, fields := range map[string]container.Bag{
		brokerredis.Driver: {},
		brokernats.Driver:  {brokernats.ConfigFieldURL: "nats://127.0.0.1:1"},
		brokerkafka.Driver: {},
	} {
		t.Run("should refuse the invalid telemetry fields of "+driver, func(t *testing.T) {
			for field, value := range map[string]any{
				broker.ConfigFieldTracing:    "yes",
				broker.ConfigFieldMetrics:    1,
				broker.ConfigFieldNamespace:  1,
				broker.ConfigFieldTopicLabel: "no",
			} {
				bag := container.Bag{field: value}
				for k, v := range fields {
					bag[k] = v
				}
				_, err := create(t, driver, bag, nil)
				var e *flam.Error
				if !errors.As(errors.Unwrap(err), &e) {
					t.Fatalf("%s: Get() = %v, want a *flam.Error", field, err)
				}
				testutil.Context(t, e, map[string]any{"id": "main", "field": field})
			}
		})

		t.Run("should fail to register the metrics of an invalid namespace with "+driver, func(t *testing.T) {
			bag := container.Bag{broker.ConfigFieldNamespace: "\xff"}
			for k, v := range fields {
				bag[k] = v
			}
			_, err := create(t, driver, bag, nil, newTelemetry().setup)
			var e *flam.Error
			if !errors.Is(err, broker.ErrRegister) || !errors.As(errors.Unwrap(err), &e) {
				t.Fatalf("Get() = %v, want ErrRegister", err)
			}
			testutil.Context(t, e, map[string]any{"id": "main"})
		})
	}

	t.Run("should unregister the metrics registered when one cannot be", func(t *testing.T) {
		tel := newTelemetry()
		testutil.NoError(t, tel.registry.Register(prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flam_broker_message_process_duration_seconds",
			Help: "taken",
		})))
		_, err := create(t, brokerkafka.Driver, container.Bag{}, nil, tel.setup, func(a *testutil.App) {
			testutil.NoError(t, a.Provide(func() flam.IBrokerKafkaClient { return newFakeKafka() }))
		})
		if !errors.Is(err, broker.ErrRegister) {
			t.Fatalf("Get() = %v, want ErrRegister", err)
		}
		families, err := tel.registry.Gather()
		testutil.NoError(t, err)
		testutil.Equal(t, "families", len(families), 1)
		testutil.Equal(t, "family", families[0].GetName(), "flam_broker_message_process_duration_seconds")
	})

	t.Run("should fail when a metric of another type holds the name", func(t *testing.T) {
		tel := newTelemetry()
		testutil.NoError(t, tel.registry.Register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flam_broker_messages_published_total",
			Help: "Number of messages published by the brokers, by broker, topic and result.",
		}, []string{broker.LabelBroker, broker.LabelTopic, broker.LabelResult})))
		_, err := create(t, brokerkafka.Driver, container.Bag{}, nil, tel.setup, func(a *testutil.App) {
			testutil.NoError(t, a.Provide(func() flam.IBrokerKafkaClient { return newFakeKafka() }))
		})
		var already prometheus.AlreadyRegisteredError
		if !errors.Is(err, broker.ErrRegister) || !errors.As(err, &already) {
			t.Fatalf("Get() = %v, want ErrRegister and AlreadyRegisteredError", err)
		}
	})
}
