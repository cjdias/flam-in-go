package redis

import (
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// creatorsIn are the connection creators the decorator decorates, of the
// ConnectionCreatorGroup group of the redis feature, and the tracer
// provider of their hook, taken from the container. The group tag repeats
// the constant, as a struct tag cannot use one.
type creatorsIn struct {
	dig.In

	Provider trace.TracerProvider
	Creators []flam.IRedisConnectionCreator `group:"flam.redis.connections.creator"`
}

// creatorsOut are the decorated connection creators, replacing the ones of
// the group.
type creatorsOut struct {
	dig.Out

	Creators []flam.IRedisConnectionCreator `group:"flam.redis.connections.creator"`
}

// decorateCreators returns the connection creators of the group, each
// adding the hook of the tracer provider to the connections it creates.
// It is the decorator registered by the bundle returned from [NewBundle].
func decorateCreators(
	in creatorsIn,
) creatorsOut {
	hook := NewHook(in.Provider)
	creators := make([]flam.IRedisConnectionCreator, len(in.Creators))
	for i, c := range in.Creators {
		creators[i] = &creator{IRedisConnectionCreator: c, hook: hook}
	}
	return creatorsOut{Creators: creators}
}

// creator is the [flam.IRedisConnectionCreator] decorating the one it
// embeds, adding the hook to the connections it creates.
type creator struct {
	flam.IRedisConnectionCreator

	hook goredis.Hook
}

// Create returns the connection created by the decorated creator, the
// hook added.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRedisConnection, error) {
	conn, err := c.IRedisConnectionCreator.Create(bag)
	if err != nil || value.IsNil(conn) {
		return conn, err
	}
	conn.AddHook(c.hook)
	return conn, nil
}
