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
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/broker/brokers]. They import this
// package for its paths, groups and errors, so this package never imports
// them.
package broker
