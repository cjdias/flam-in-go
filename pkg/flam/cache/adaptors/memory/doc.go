// Package memory is the in-process cache adaptor driver: its adaptors
// cache the objects in a map of the process, lost when it exits, and not
// shared with the other processes nor with the other adaptors, each
// holding its own. [NewAdaptor] creates one from a key generator, a
// serializer and a clock taken elsewhere.
//
// As the redis adaptors do, an adaptor stores each object under the key
// of its key generator, as the string of its serializer, so that Get fills
// the objects with copies of the cached values, never sharing them with
// the objects given to Set.
//
// An adaptor holds at most the number of values of its max entries field,
// or of its default, evicting the least recently used one, set or got,
// to store another. An ephemeral value expires once its ttl elapsed on the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container: it is
// removed when found expired by a call, or by the sweep a write runs once
// per sweep period, removing every expired value.
package memory
