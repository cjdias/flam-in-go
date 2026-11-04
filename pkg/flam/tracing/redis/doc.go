// Package redis traces the commands of the redis connections of
// [github.com/cjdias/flam-in-go/pkg/flam/redis] with the
// [go.opentelemetry.io/otel/trace.TracerProvider] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing].
//
// The go-redis hook returned by [NewHook] runs each command of the
// clients it is added to within a client span, a child of the span of the
// context of the command, named by the command, such as "redis.get", and
// each pipeline within a "redis.pipeline" span. The bundle returned from
// [NewBundle] adds it to every connection created by the connection
// factory; it can also be added to a connection, or to any go-redis
// client, with AddHook:
//
//	conn.AddHook(redis.NewHook(provider))
//
// The spans have a parent only when the command is given the context of
// the caller, such as the context of a request served within the span of
// the REST middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware]:
//
//	value, err := conn.Get(req.Context(), "key").Result()
//
// The arguments of the commands are left out of the spans, as they may
// hold private data.
package redis
