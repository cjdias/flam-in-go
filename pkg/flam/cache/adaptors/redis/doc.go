// Package redis is the cache adaptor driver of Redis: its adaptors cache
// the objects in the Redis server of a connection of the
// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory], named
// by its id in their connection field, or its default. [NewAdaptor]
// creates one from a key generator, a serializer and a connection taken
// elsewhere.
//
// The adaptors of one server share its keys: give their key generators
// different prefixes, or their connections different databases, to keep
// them apart.
//
// The adaptors implement
// [github.com/cjdias/flam-in-go/pkg/flam.ICacheKeyedAdaptor], telling the
// keys of the objects and removing the values by key, with DEL.
package redis
