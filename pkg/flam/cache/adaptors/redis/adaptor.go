package redis

import (
	"context"
	"reflect"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewAdaptor returns the [flam.ICacheAdaptor] of the id caching the
// objects in the Redis server of the connection, under the keys of the
// key generator, as the strings of the serializer. It fails with
// ErrNilKeyGenerator, ErrNilSerializer or ErrNilConnection when one of
// them is nil.
//
// Has checks the keys in one pipeline, Get reads them with MGET, Set
// writes them with MSET, SetEphemeral in one MULTI/EXEC transaction,
// and Del removes them with DEL, so that each call is one round trip, and
// the writes are atomic. The commands run with the context given, its
// cancellation failing the call with an error wrapping
// [github.com/cjdias/flam-in-go/pkg/flam/cache.ErrStorage], within the
// timeouts of the connection.
func NewAdaptor(
	id string,
	keyGenerator flam.ICacheKeyGenerator,
	serializer flam.ICacheSerializer,
	connection flam.IRedisConnection,
) (flam.ICacheAdaptor, error) {
	switch {
	case value.IsNil(keyGenerator):
		return nil, newErrNil(ErrNilKeyGenerator, adaptorNewMessage, id)
	case value.IsNil(serializer):
		return nil, newErrNil(ErrNilSerializer, adaptorNewMessage, id)
	case value.IsNil(connection):
		return nil, newErrNil(ErrNilConnection, adaptorNewMessage, id)
	}
	return &adaptor{
		id:           id,
		keyGenerator: keyGenerator,
		serializer:   serializer,
		connection:   connection,
	}, nil
}

// adaptor is the [flam.ICacheAdaptor] returned by [NewAdaptor].
type adaptor struct {
	id           string
	keyGenerator flam.ICacheKeyGenerator
	serializer   flam.ICacheSerializer
	connection   flam.IRedisConnection
}

func (a *adaptor) Id() string {
	return a.id
}

func (a *adaptor) Has(
	ctx context.Context,
	objects ...any,
) ([]bool, error) {
	keys, err := a.keys(adaptorHasMessage, objects)
	if err != nil || len(keys) == 0 {
		return []bool{}, err
	}

	ctx = orBackground(ctx)
	cmds := make([]*goredis.IntCmd, len(keys))
	if _, err := a.connection.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		for i, key := range keys {
			cmds[i] = pipe.Exists(ctx, key)
		}
		return nil
	}); err != nil {
		return nil, newErrStorage(adaptorHasMessage, a.id, err)
	}

	found := make([]bool, len(keys))
	for i, cmd := range cmds {
		found[i] = cmd.Val() > 0
	}
	return found, nil
}

func (a *adaptor) Get(
	ctx context.Context,
	objects ...any,
) ([]any, error) {
	keys, err := a.keys(adaptorGetMessage, objects)
	if err != nil || len(keys) == 0 {
		return []any{}, err
	}

	values, err := a.connection.MGet(orBackground(ctx), keys...).Result()
	if err != nil {
		return nil, newErrStorage(adaptorGetMessage, a.id, err)
	}

	// decoded in new values, set once all are, so that a failure changes no
	// object, and a value replaces the whole object, instead of merging in
	fresh := make([]reflect.Value, len(objects))
	for i, val := range values {
		data, ok := val.(string)
		if !ok { // nil for a missing key
			continue
		}
		target := objects[i]
		if rv := reflect.ValueOf(target); rv.Kind() == reflect.Pointer && !rv.IsNil() {
			fresh[i] = reflect.New(rv.Type().Elem())
			target = fresh[i].Interface()
		}
		if err := a.serializer.Deserialize(data, target); err != nil {
			return nil, newErrObject(adaptorGetMessage, a.id, keys[i], err)
		}
	}

	found := make([]any, len(objects))
	for i, value := range fresh {
		if value.IsValid() {
			reflect.ValueOf(objects[i]).Elem().Set(value.Elem())
			found[i] = objects[i]
		}
	}
	return found, nil
}

func (a *adaptor) Set(
	ctx context.Context,
	objects ...any,
) error {
	pairs, err := a.pairs(adaptorSetMessage, objects)
	if err != nil || len(pairs) == 0 {
		return err
	}

	if err := a.connection.MSet(orBackground(ctx), pairs...).Err(); err != nil {
		return newErrStorage(adaptorSetMessage, a.id, err)
	}
	return nil
}

func (a *adaptor) SetEphemeral(
	ctx context.Context,
	ttl time.Duration,
	objects ...any,
) error {
	if ttl <= 0 {
		return newErrNil(ErrInvalidTTL, adaptorSetEphemeralMessage, a.id)
	}
	pairs, err := a.pairs(adaptorSetEphemeralMessage, objects)
	if err != nil || len(pairs) == 0 {
		return err
	}

	ctx = orBackground(ctx)
	if _, err := a.connection.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		for i := 0; i < len(pairs); i += 2 {
			pipe.Set(ctx, pairs[i].(string), pairs[i+1], ttl)
		}
		return nil
	}); err != nil {
		return newErrStorage(adaptorSetEphemeralMessage, a.id, err)
	}
	return nil
}

func (a *adaptor) Del(
	ctx context.Context,
	objects ...any,
) error {
	keys, err := a.keys(adaptorDelMessage, objects)
	if err != nil || len(keys) == 0 {
		return err
	}

	if err := a.connection.Del(orBackground(ctx), keys...).Err(); err != nil {
		return newErrStorage(adaptorDelMessage, a.id, err)
	}
	return nil
}

// Messages of the errors returned by NewAdaptor and the adaptor methods.
const (
	adaptorNewMessage          = "unable to create redis cache adaptor"
	adaptorHasMessage          = "unable to check redis cache"
	adaptorGetMessage          = "unable to get from redis cache"
	adaptorSetMessage          = "unable to set in redis cache"
	adaptorSetEphemeralMessage = "unable to set ephemeral in redis cache"
	adaptorDelMessage          = "unable to delete from redis cache"
)

// orBackground returns the context, or context.Background when nil.
func orBackground(
	ctx context.Context,
) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// keys returns the keys of the objects, failing with the error of the
// first one the key generator fails for.
func (a *adaptor) keys(
	message string,
	objects []any,
) ([]string, error) {
	keys := make([]string, len(objects))
	for i, obj := range objects {
		key, err := a.keyGenerator.Generate(obj)
		if err != nil {
			return nil, newErrObject(message, a.id, "", err)
		}
		keys[i] = key
	}
	return keys, nil
}

// pairs returns the keys of the objects, each followed by the string of
// its object, failing with the error of the first one the key generator
// or the serializer fails for.
func (a *adaptor) pairs(
	message string,
	objects []any,
) ([]any, error) {
	keys, err := a.keys(message, objects)
	if err != nil {
		return nil, err
	}
	pairs := make([]any, 0, 2*len(keys))
	for i, key := range keys {
		data, err := a.serializer.Serialize(objects[i])
		if err != nil {
			return nil, newErrObject(message, a.id, key, err)
		}
		pairs = append(pairs, key, data)
	}
	return pairs, nil
}
