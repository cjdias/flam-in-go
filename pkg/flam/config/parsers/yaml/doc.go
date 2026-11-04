// Package yaml is the configuration parser driver of the YAML documents:
// its parsers decode a YAML mapping into a
// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag], with
// [go.yaml.in/yaml/v3], which follows YAML 1.2.
//
// The content must hold a single YAML document, a mapping, or null, or
// nothing but whitespace and comments, which give an empty bag. Its values
// become:
//   - mappings: a container.Bag, whose keys are the text of the YAML keys,
//     so that 1, true or ~ keys give the "1", "true" and "~" keys;
//   - sequences: a []any;
//   - integers, decimal or not, such as 8080, 0x1F or 1_000: an int when
//     they fit in an int, so that the int getters of the configuration read
//     them, or a float64 otherwise;
//   - floats, such as 1.0, 1e2, .inf, -.inf or .nan: a float64, a float
//     beyond the float64 range, such as 1e400, being kept as a string;
//   - timestamps, such as 2001-12-14: the string written, as for JSON;
//   - strings, booleans and null: a string, a bool and nil.
//
// Anchors, aliases and merge keys (<<) are resolved, each alias giving a
// copy of its anchored value. Unlike the JSON parser, a mapping holding the
// same key twice is refused, as YAML requires unique keys.
//
// A key holding a dot, such as "a.b", is kept as a single key of the
// parsed bag, which a dot separated path cannot address, as the response
// path of a rest source; the configuration expands it into nested values
// when it merges the source.
package yaml
