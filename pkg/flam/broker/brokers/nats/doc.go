// Package nats is the broker driver of a NATS server, each broker owning
// its connection, in one of two modes, held by the jetstream field:
//
//   - core NATS, the default, publishes on the subject of the topic and
//     subscribes to it, the topic of a subscription holding the NATS
//     wildcards, if any, delivering each message at most once to every
//     subscription in place when it is published, or to one subscription
//     of the queue group of the queue field: a message published while no
//     subscription is connected, or whose handler fails, is lost. The
//     connection reconnects, and subscribes again, on its own;
//   - JetStream stores the messages in the stream of the stream field,
//     created or updated with the subjects field when set, and subscribes
//     with a durable consumer per topic, shared by the subscriptions of the
//     brokers of the same durable field, each message going to one of
//     them, delivering each message published from its creation at least
//     once: a message whose handler succeeds is acknowledged, while a
//     message whose handler fails is asked to be delivered again, as is a
//     message left unacknowledged for the ack_wait field, up to the
//     max_deliver field.
//
// A broker connects when created, failing when the server is unreachable.
// A subscription calls its handler one message at a time.
package nats
