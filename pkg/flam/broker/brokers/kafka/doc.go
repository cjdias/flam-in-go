// Package kafka is the broker driver of a Kafka cluster, with
// [github.com/segmentio/kafka-go]: each broker owns a writer publishing
// the messages in batches, configured by its batch_size, batch_bytes,
// batch_timeout, required_acks and async fields, and each subscription a
// reader of the consumer group of the group_id field, configured by its
// min_bytes, max_bytes, max_wait and start_offset fields. Neither reaches
// the cluster when created: the writer connects on its first write, and
// the reader joins its group in the background, so that, with the "last"
// start offset, a new group misses the messages published before it
// joined.
//
// It delivers each message at least once to one subscription of each
// group, in the order of its partition: a message whose handler fails is
// handled again, after the retry_backoff field, until its handler
// succeeds, holding back the messages after it in its partition, and its
// offset is committed only then. A message handled but not committed, as
// its subscription stopped first, is delivered again to the group.
//
// The [HeaderKey] header carries the key of the messages. The brokers use
// the [github.com/cjdias/flam-in-go/pkg/flam.IBrokerKafkaClient] of the
// application container, when provided, such as a fake in tests, and
// kafka-go otherwise.
package kafka
