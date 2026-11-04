// Package redis is the broker driver of a Redis server, reached over a
// connection of the
// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory], which
// keeps owning it, in one of two modes, held by the mode field:
//
//   - [ModePubSub] publishes with PUBLISH on the channel of the topic, and
//     subscribes with SUBSCRIBE, delivering each message at most once to
//     every subscription in place when it is published: a message
//     published while a subscription is disconnected, or whose handler
//     fails, is lost. The payload and the headers travel in a JSON
//     envelope, a message of another publisher being delivered whole as
//     the payload. The subscriptions reconnect and subscribe again on
//     their own, ending with the error wrapping
//     [github.com/cjdias/flam-in-go/pkg/flam/broker.ErrReceive] once the
//     connection is closed;
//   - [ModeStreams] adds the messages to the stream of the topic with
//     XADD, trimmed to the max_len field, and subscribes with XREADGROUP
//     in the consumer group of the group field, created along with its
//     stream when missing, delivering each message at least once to one
//     consumer of each group: an entry whose handler succeeds is
//     acknowledged with XACK, while an entry whose handler fails, or whose
//     consumer stops, stays pending, to be claimed with XAUTOCLAIM by a
//     subscription of its group, and delivered again, once idle for the
//     claim_idle field, a claim_idle of 0 leaving it pending for good. A
//     subscription without the consumer field reads as a consumer of its
//     own, deleted from the group with XGROUP DELCONSUMER when it ends,
//     unless it holds pending entries, kept for them to be claimed.
//
// A subscription reads its messages in a goroutine of its own, calling
// its handler one message at a time. In the streams mode, its reads wait
// for new entries for the block field at most, which bounds the time
// closing it takes.
package redis
