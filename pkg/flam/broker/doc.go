// Package broker is the broker feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IBrokerFactory] of an
// application, which creates the brokers described by the configuration
// at [BrokerConfigPath] with the creators provided in the
// [BrokerCreatorGroup] container group, such as:
//
//	flam:
//	  broker:
//	    brokers:
//	      events: {driver: flam.broker.brokers.driver.nats, url: "nats://nats:4222", queue: orders}
//	      jobs:   {driver: flam.broker.brokers.driver.redis, connection: main, mode: streams}
//	      audit:  {driver: flam.broker.brokers.driver.kafka, brokers: ["kafka:9092"], group_id: audit}
//
// A broker publishes opaque payloads, with string headers, on topics, and
// subscribes handlers to them, each subscription calling its handler from
// a goroutine of its own, one message at a time. A handler returning nil
// acknowledges its message, while a handler returning an error, or
// panicking, leaves it unacknowledged. The delivery semantics depend on
// the driver, each documenting its own:
//
//   - Redis Pub/Sub and core NATS deliver at most once, to the
//     subscriptions in place when the message is published: a failed
//     message is lost;
//   - Redis Streams, NATS JetStream and Kafka deliver at least once, to
//     one consumer of each group: a failed message is delivered again, so
//     that the handlers must be idempotent.
//
// [PublishJSON] and [SubscribeJSON] carry typed values as JSON, and
// [NewProcess] runs a subscription as a process of the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog], so that a long-lived
// consumer starts and stops with the application. [NewMessage] makes the
// messages of the drivers, and of the tests of the handlers.
//
// The brokers of every driver trace and measure their messages, unless
// disabled by their [ConfigFieldTracing] and [ConfigFieldMetrics] fields,
// when the container provides the
// [go.opentelemetry.io/otel/trace.TracerProvider] and the
// [go.opentelemetry.io/otel/propagation.TextMapPropagator] of the bundle
// of [github.com/cjdias/flam-in-go/pkg/flam/tracing], and the Prometheus
// registerer of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics], and work as well
// without them. A message is published within a producer span, named
// "publish" followed by its topic, a child of the span of the context
// given to Publish, whose context is injected into the headers of the
// message, such as the "traceparent" header of the W3C trace context,
// which travel with it, the headers given being left as is. Each
// delivery, or retry, of a message to a handler runs within a consumer
// span, named "process" followed by the topic of the subscription, the
// child of the span extracted from its headers, so that the trace goes on
// from the publisher to the subscriber, and the context given to the
// handler holds it. The spans hold the attributes of the messaging
// semantic conventions of OpenTelemetry: the system, such as "redis",
// "nats" or "kafka", the topic, the operation, its type being "send" or
// "process", and the id and size of the message, a backend refusing the
// message, or a handler returning an error or panicking, recording its
// error and setting the status of the span to an error.
//
// The publications are counted on [MetricPublished], and the handlings on
// [MetricProcessed], their time observed on [MetricProcessDuration],
// named after the namespace of the broker, "flam" unless set, such as
// "flam_broker_messages_published_total". They are labelled by the id of
// the broker at [LabelBroker], the topic at [LabelTopic], the topic of the
// subscription for a handling, such as the wildcard subject of a NATS
// subscription, and the result at [LabelResult]. The brokers of the same
// namespace share their metrics. As each topic is a series of each
// metric, a broker of many topics, such as topics made of ids, should
// leave the topic out of its metrics with its [ConfigFieldTopicLabel]
// field:
//
//	flam:
//	  broker:
//	    brokers:
//	      events: {driver: flam.broker.brokers.driver.nats, tracing: true, topic_label: false}
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/broker/brokers]. They import this
// package for its paths, groups and errors, so this package never imports
// them.
package broker
