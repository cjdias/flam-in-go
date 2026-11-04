package broker

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.broker"

// Configuration path and container group of the brokers.
const (
	// BrokerConfigPath is the configuration path holding one broker
	// configuration per broker id, as read by the broker factory.
	BrokerConfigPath = "flam.broker.brokers"

	// BrokerCreatorGroup is the container group the broker driver bundles
	// provide their [flam.IBrokerCreator] in, collected by the broker
	// factory.
	BrokerCreatorGroup = "flam.broker.brokers.creator"
)

// Fields of a broker configuration shared by every driver, tracing and
// measuring the messages of the broker.
const (
	// ConfigFieldTracing is the field holding the bool telling whether the
	// broker traces its messages, which it does only when the container
	// provides a tracer provider and a propagator. It defaults to true.
	ConfigFieldTracing = "tracing"

	// ConfigFieldMetrics is the field holding the bool telling whether the
	// broker measures its messages, which it does only when the container
	// provides a Prometheus registerer. It defaults to true.
	ConfigFieldMetrics = "metrics"

	// ConfigFieldNamespace is the field holding the namespace of the
	// metrics of the broker, the first part of their names, a string,
	// empty for none. It defaults to "flam".
	ConfigFieldNamespace = "namespace"

	// ConfigFieldTopicLabel is the field holding the bool telling whether
	// the metrics of the broker are labelled by topic, at [LabelTopic],
	// which is otherwise left empty, as Prometheus takes a label of no
	// value as missing, so as to bound the number of series of a broker of
	// many topics. It defaults to true.
	ConfigFieldTopicLabel = "topic_label"
)

// Configuration paths of the values taken by the shared fields missing
// from a broker configuration, whatever its driver, unset unless set by
// the application: the built-in defaults then apply.
const (
	// DefaultTracingConfigPath is the path of the default of
	// [ConfigFieldTracing].
	DefaultTracingConfigPath = "flam.broker.defaults.brokers.tracing"

	// DefaultMetricsConfigPath is the path of the default of
	// [ConfigFieldMetrics].
	DefaultMetricsConfigPath = "flam.broker.defaults.brokers.metrics"

	// DefaultNamespaceConfigPath is the path of the default of
	// [ConfigFieldNamespace].
	DefaultNamespaceConfigPath = "flam.broker.defaults.brokers.namespace"

	// DefaultTopicLabelConfigPath is the path of the default of
	// [ConfigFieldTopicLabel].
	DefaultTopicLabelConfigPath = "flam.broker.defaults.brokers.topic_label"
)

// Tracing of the messages of the brokers.
const (
	// ScopeName is the name of the instrumentation scope of the spans of
	// the brokers, the name of the tracer they start them with.
	ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/broker"

	// OperationPublish is the operation of the producer spans of the
	// published messages, held by their "messaging.operation.name"
	// attribute, and starting their names, followed by the topic, such as
	// "publish orders".
	OperationPublish = "publish"

	// OperationProcess is the operation of the consumer spans of the
	// handled messages, held by their "messaging.operation.name"
	// attribute, and starting their names, followed by the topic of the
	// subscription, such as "process orders.*".
	OperationProcess = "process"
)

// Names of the metrics of the brokers, following the namespace and the
// [Subsystem].
const (
	// MetricPublished is the name of the counter of the messages
	// published.
	MetricPublished = "messages_published_total"

	// MetricProcessed is the name of the counter of the messages handed to
	// the handlers, each delivery and each retry counting.
	MetricProcessed = "messages_processed_total"

	// MetricProcessDuration is the name of the histogram of the time taken
	// by the handlers to handle the messages, in seconds.
	MetricProcessDuration = "message_process_duration_seconds"

	// Subsystem is the subsystem of the metrics, following their
	// namespace in their names, such as
	// "flam_broker_messages_published_total".
	Subsystem = "broker"
)

// Labels of the metrics of the brokers.
const (
	// LabelBroker holds the id of the broker.
	LabelBroker = "broker"

	// LabelTopic holds the topic a message was published on, or the topic
	// of the subscription handling it, which, for a subscription to a NATS
	// wildcard subject, is the wildcard subject, or nothing without
	// [ConfigFieldTopicLabel].
	LabelTopic = "topic"

	// LabelResult holds the result of the publication or of the handling
	// of a message: [ResultSuccess], [ResultError] or [ResultPanic].
	LabelResult = "result"
)

// Results of the publications and of the handlings of the messages, held
// by [LabelResult].
const (
	// ResultSuccess is the result of a message published, or handled
	// without error.
	ResultSuccess = "success"

	// ResultError is the result of a message the backend refused, or whose
	// handler returned an error.
	ResultError = "error"

	// ResultPanic is the result of a message whose handler panicked.
	ResultPanic = "panic"
)

// Context fields of the [*flam.Error] values returned by the broker
// package and the broker drivers.
const (
	// ErrorFieldID holds the id of a broker, or of a process.
	ErrorFieldID = "id"

	// ErrorFieldTopic holds the topic of a publication or of a
	// subscription.
	ErrorFieldTopic = "topic"

	// ErrorFieldMessage holds the id of a message, as given by the
	// backend.
	ErrorFieldMessage = "message"

	// ErrorFieldPanic holds the value a handler panicked with.
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack trace of the goroutine of a handler
	// when it panicked, as a string.
	ErrorFieldStack = "stack"
)
