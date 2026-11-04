// Package json is the configuration parser driver of the JSON documents:
// its parsers decode a JSON object into a
// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag].
//
// The document must be a single JSON object, or null, or empty, which give
// an empty bag. Its values become:
//   - objects: a container.Bag;
//   - arrays: a []any;
//   - numbers: an int when they are whole and fit in an int, so that the
//     int getters of the configuration read them, or a float64 otherwise,
//     rounded to the nearest float64, or to ±Inf when out of its range;
//   - strings, booleans and null: a string, a bool and nil.
//
// When an object holds the same key twice, the last value wins.
//
// A key holding a dot, such as "a.b", is kept as a single key of the
// parsed bag, which a dot separated path cannot address, as the response
// path of a rest source; the configuration expands it into nested values
// when it merges the source.
package json
