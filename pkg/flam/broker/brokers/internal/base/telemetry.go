package base

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
)

// TelemetryArgs are the dependencies of the telemetry of the brokers,
// taken from the container by the creators of the drivers. They are
// optional, so that the brokers work without the tracing and the metrics
// features: their absence only leaves the messages untraced, or
// unmeasured.
type TelemetryArgs struct {
	dig.In

	Provider   trace.TracerProvider          `optional:"true"`
	Propagator propagation.TextMapPropagator `optional:"true"`
	Registerer prometheus.Registerer         `optional:"true"`
}

// TelemetryConfig is the configuration of the telemetry of a broker, read
// by the Telemetry method of a Reader.
type TelemetryConfig struct {
	Tracing    bool
	Metrics    bool
	Namespace  string
	TopicLabel bool
}

// Telemetry reads the fields of the telemetry of the broker, shared by
// every driver, or their defaults.
func (r *Reader) Telemetry() TelemetryConfig {
	return TelemetryConfig{
		Tracing:    r.Bool(broker.ConfigFieldTracing, broker.DefaultTracingConfigPath, defaultTracing),
		Metrics:    r.Bool(broker.ConfigFieldMetrics, broker.DefaultMetricsConfigPath, defaultMetrics),
		Namespace:  r.String(broker.ConfigFieldNamespace, broker.DefaultNamespaceConfigPath, defaultNamespace, false),
		TopicLabel: r.Bool(broker.ConfigFieldTopicLabel, broker.DefaultTopicLabelConfigPath, defaultTopicLabel),
	}
}

// NewTelemetry returns the telemetry of the broker of the id and of the
// messaging system, such as "redis", tracing its messages, unless
// disabled, when the args hold a tracer provider and a propagator, and
// measuring them, unless disabled, when they hold a registerer, after
// registering its metrics, or taking those registered already by a broker
// of the same namespace. When one of its metrics cannot be registered,
// those it registered are unregistered, and the error of the registerer
// is returned.
func NewTelemetry(
	args TelemetryArgs,
	config TelemetryConfig,
	id string,
	system string,
) (*Telemetry, error) {
	t := &Telemetry{
		id:         id,
		system:     semconv.MessagingSystemKey.String(system),
		topicLabel: config.TopicLabel,
	}
	if config.Tracing && args.Provider != nil && args.Propagator != nil {
		t.tracer = args.Provider.Tracer(broker.ScopeName)
		t.propagator = args.Propagator
	}
	if config.Metrics && args.Registerer != nil {
		labels := []string{broker.LabelBroker, broker.LabelTopic, broker.LabelResult}
		r := &registration{registerer: args.Registerer}
		t.published = register(r, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: broker.Subsystem,
			Name:      broker.MetricPublished,
			Help:      telemetryPublishedHelp,
		}, labels))
		t.processed = register(r, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: broker.Subsystem,
			Name:      broker.MetricProcessed,
			Help:      telemetryProcessedHelp,
		}, labels))
		t.duration = register(r, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: broker.Subsystem,
			Name:      broker.MetricProcessDuration,
			Help:      telemetryDurationHelp,
			Buckets:   prometheus.DefBuckets,
		}, labels))
		if r.err != nil {
			r.rollback()
			return nil, r.err
		}
	}
	return t, nil
}

// Telemetry traces and measures the messages of a broker, as configured.
// It is safe for concurrent use.
type Telemetry struct {
	id         string
	system     attribute.KeyValue
	topicLabel bool
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	published  *prometheus.CounterVec
	processed  *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

// Publish calls publish with the context and the headers of the message
// published on the topic, returning its error. With a tracer, publish
// runs within a producer span, a child of the span of the context, named
// "publish" followed by the topic, whose context is injected into a copy
// of the headers given to publish, the headers given being left as is,
// and the context given to publish holds the span. A nil context is then
// taken as the background one. With metrics, the publication is counted,
// by its result.
func (t *Telemetry) Publish(
	ctx context.Context,
	topic string,
	headers map[string]string,
	publish func(ctx context.Context, headers map[string]string) error,
) error {
	span := trace.SpanFromContext(context.Background())
	if t.tracer != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, span = t.tracer.Start(ctx, broker.OperationPublish+" "+topic,
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				t.system,
				semconv.MessagingDestinationName(topic),
				semconv.MessagingOperationTypeSend,
				semconv.MessagingOperationName(broker.OperationPublish),
			),
		)
		injected := make(map[string]string, len(headers)+telemetryInjectedHeaders)
		maps.Copy(injected, headers)
		t.propagator.Inject(ctx, propagation.MapCarrier(injected))
		headers = injected
	}
	err := publish(ctx, headers)
	t.end(span, err, false)
	if t.published != nil {
		t.published.WithLabelValues(t.id, t.topic(topic), telemetryResult(err, false)).Inc()
	}
	return err
}

// Handle calls the handler of the subscription to the topic with the
// message, as the call function does, returning its error. With a tracer,
// the handler runs within a consumer span, a child of the span of the
// context the propagator extracts from the headers of the message, if
// any, or else of the span of the context, named "process" followed by
// the topic of the subscription, and the context given to the handler
// holds the span. With metrics, the handling is counted, and its time
// observed, by its result.
func (t *Telemetry) Handle(
	ctx context.Context,
	topic string,
	handler flam.BrokerHandler,
	msg flam.IBrokerMessage,
) error {
	span := trace.SpanFromContext(context.Background())
	if t.tracer != nil {
		attributes := []attribute.KeyValue{
			t.system,
			semconv.MessagingDestinationName(msg.Topic()),
			semconv.MessagingOperationTypeProcess,
			semconv.MessagingOperationName(broker.OperationProcess),
			semconv.MessagingMessageBodySize(len(msg.Payload())),
		}
		if topic != msg.Topic() {
			attributes = append(attributes, semconv.MessagingDestinationTemplate(topic))
		}
		if msg.Id() != "" {
			attributes = append(attributes, semconv.MessagingMessageID(msg.Id()))
		}
		ctx = t.propagator.Extract(ctx, propagation.MapCarrier(msg.Headers()))
		ctx, span = t.tracer.Start(ctx, broker.OperationProcess+" "+topic,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attributes...),
		)
	}
	start := time.Now()
	panicked, err := call(ctx, t.id, handler, msg)
	elapsed := time.Since(start)
	t.end(span, err, panicked)
	if t.processed != nil {
		result := telemetryResult(err, panicked)
		t.processed.WithLabelValues(t.id, t.topic(topic), result).Inc()
		t.duration.WithLabelValues(t.id, t.topic(topic), result).Observe(elapsed.Seconds())
	}
	return err
}

// Defaults of the fields of the telemetry of a broker.
const (
	defaultTracing    = true
	defaultMetrics    = true
	defaultNamespace  = "flam"
	defaultTopicLabel = true
)

// Help texts of the metrics.
const (
	telemetryPublishedHelp = "Number of messages published by the brokers, by broker, topic and result."
	telemetryProcessedHelp = "Number of messages handled by the subscriptions of the brokers, by broker, topic and result."
	telemetryDurationHelp  = "Time taken by the subscriptions of the brokers to handle the messages, in seconds, by broker, topic and result."
)

// telemetryInjectedHeaders is the room made for the headers injected by
// the propagator, such as "traceparent", "tracestate" and "baggage".
const telemetryInjectedHeaders = 3

// end records the error on the span, unless nil, then ends it.
func (t *Telemetry) end(
	span trace.Span,
	err error,
	panicked bool,
) {
	defer span.End()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(semconv.ErrorTypeKey.String(telemetryResult(err, panicked)))
		span.SetStatus(codes.Error, err.Error())
	}
}

// topic returns the value of the topic label of the topic, empty without
// topic label.
func (t *Telemetry) topic(
	topic string,
) string {
	if !t.topicLabel {
		return ""
	}
	return topic
}

// telemetryResult returns the result of the error: ResultPanic when the
// handler panicked, or else ResultError, unless nil, ResultSuccess.
func telemetryResult(
	err error,
	panicked bool,
) string {
	switch {
	case panicked:
		return broker.ResultPanic
	case err != nil:
		return broker.ResultError
	}
	return broker.ResultSuccess
}

// registration registers the metrics of a broker, keeping the first
// error, after which it registers nothing, and the metrics it registered,
// to unregister them on failure.
type registration struct {
	registerer prometheus.Registerer
	registered []prometheus.Collector
	err        error
}

// rollback unregisters the metrics registered.
func (r *registration) rollback() {
	for _, collector := range r.registered {
		r.registerer.Unregister(collector)
	}
}

// register registers the metric, returning it, or the metric of the same
// type registered already under the same description, or the zero value
// once the registration failed.
func register[C prometheus.Collector](
	r *registration,
	collector C,
) C {
	var zero C
	if r.err != nil {
		return zero
	}
	err := r.registerer.Register(collector)
	if err == nil {
		r.registered = append(r.registered, collector)
		return collector
	}
	var registered prometheus.AlreadyRegisteredError
	if errors.As(err, &registered) {
		if existing, ok := registered.ExistingCollector.(C); ok {
			return existing
		}
	}
	r.err = err
	return zero
}
