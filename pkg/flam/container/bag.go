// Package container provides Bag, a tree of loosely typed values addressed
// by dot separated paths, such as configuration data.
package container

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// Bag is a tree of named values, typically used to hold configuration or
// other loosely structured data, addressed by dot separated paths.
//
// # Paths
//
// A path is a sequence of keys joined by [BagPathSeparator], such as
// "database.primary.port". Each segment but the last names a nested bag,
// and the last one names the addressed value. The empty path "" addresses
// the bag itself. A path with an empty segment, such as "a..b", ".a" or
// "a.", is malformed: lookups treat it as not found, and modifications
// fail with [ErrInvalidPath]. A key containing the separator can be
// stored in a bag directly, as a map key, but cannot be addressed by a
// path; [Bag.Merge], however, expands such keys into nested bags.
//
// # Nested bags
//
// Bag and map[string]any values are treated as nested bags when a path is
// followed, so data decoded by encoding/json or gopkg.in/yaml.v3 can be
// stored and navigated without conversion. Bags created by the bag itself,
// such as the intermediate levels made by [Bag.Set], are of type Bag. A
// map[any]any, as produced by gopkg.in/yaml.v2, is an ordinary value
// except when merged with [Bag.Merge], which converts it into a Bag.
//
// A nil value, or a nil Bag or map[string]any, is an empty slot: it does
// not hold any nested value when read, and is replaced with a new Bag when
// a modification needs a bag at its place.
//
// A bag must not contain itself, directly or through nested values:
// [Bag.Clone], [Bag.Merge] and the [flam.Error] context copy values
// recursively and do not terminate on such a bag, as with
// encoding/json.
//
// # Reading values
//
// [Bag.Has] and [Bag.Get] report and return values of any type. The typed
// getters, such as [Bag.Int] or [Bag.String], return the value converted to
// a specific type and take an optional default, returned when the path does
// not exist or when the value is not of the requested type. When no default
// is given, the zero value of the type is returned instead. Only the first
// default is used; any extra ones are ignored.
//
// Except where documented otherwise, the typed getters perform no
// conversion: the stored value must have exactly the requested type. For
// example, [Bag.Int] does not accept an int64, a float64 (as produced by
// encoding/json for every number) or a named type based on int. To tell a
// missing value apart from a zero or default one, use [Bag.Has].
//
// [Bag.All] iterates over the top level entries, in key order.
//
// Getters return stored maps and slices without copying them, so changes
// made through the result are visible in the bag. Use [Bag.Clone] to obtain
// an independent copy.
//
// # Concurrency
//
// A Bag is a Go map and shares its concurrency rules: any number of
// goroutines may read it at the same time, but a modification, through
// [Bag.Set], [Bag.Delete], [Bag.Merge] or directly, must not happen
// concurrently with any other access.
type Bag map[string]any

// compile time check that Bag implements flam.IBag
var _ flam.IBag = Bag(nil)

// Has reports whether a value is stored at the given path.
//
// A value is considered stored even when it is nil, so Has can be used to
// tell a missing value from a nil, zero or default one. The empty path
// always exists, as it addresses the bag itself. A malformed path, or a path
// going through a value that is not a bag, does not exist.
//
//	b := container.Bag{"a": container.Bag{"b": nil}}
//
//	b.Has("a")     // true
//	b.Has("a.b")   // true, the value is nil but present
//	b.Has("a.c")   // false
//	b.Has("a.b.c") // false, "a.b" is not a bag
func (b Bag) Has(
	path string,
) bool {
	_, ok := b.lookup(path)
	return ok
}

// Get returns the value stored at the given path, whatever its type.
//
// When no value is stored at the path, Get returns the given default, or nil
// when none is given. A stored nil value is returned as nil, not replaced
// by the default. The empty path returns the bag itself.
//
// The value is returned as stored, without copying, so a returned map or
// slice shares its contents with the bag.
//
//	b.Get("server.host")              // the stored value, or nil
//	b.Get("server.host", "localhost") // the stored value, or "localhost"
func (b Bag) Get(
	path string,
	def ...any,
) any {
	if v, ok := b.lookup(path); ok {
		return v
	}
	return bagFirstDef(def)
}

// Bool returns the bool stored at the given path.
//
// When no value is stored at the path, or the stored value is not a bool,
// Bool returns the given default, or false when none is given. Strings such
// as "true" or numbers such as 1 are not converted.
func (b Bag) Bool(
	path string,
	def ...bool,
) bool {
	return bagGet(b, path, def)
}

// Int returns the int stored at the given path.
//
// When no value is stored at the path, or the stored value is not an int,
// Int returns the given default, or 0 when none is given. Values of other
// numeric types, including the other integer sizes and the float64 numbers
// produced by encoding/json, are not converted.
func (b Bag) Int(
	path string,
	def ...int,
) int {
	return bagGet(b, path, def)
}

// Int8 returns the int8 stored at the given path.
//
// When no value is stored at the path, or the stored value is not an int8,
// Int8 returns the given default, or 0 when none is given. Values of other
// numeric types are not converted.
func (b Bag) Int8(
	path string,
	def ...int8,
) int8 {
	return bagGet(b, path, def)
}

// Int16 returns the int16 stored at the given path.
//
// When no value is stored at the path, or the stored value is not an int16,
// Int16 returns the given default, or 0 when none is given. Values of other
// numeric types are not converted.
func (b Bag) Int16(
	path string,
	def ...int16,
) int16 {
	return bagGet(b, path, def)
}

// Int32 returns the int32 stored at the given path.
//
// When no value is stored at the path, or the stored value is not an int32,
// Int32 returns the given default, or 0 when none is given. Values of other
// numeric types are not converted.
//
// Since rune is an alias of int32, Int32 and [Bag.Rune] are
// interchangeable: a rune stored in the bag is returned by both.
func (b Bag) Int32(
	path string,
	def ...int32,
) int32 {
	return bagGet(b, path, def)
}

// Int64 returns the int64 stored at the given path.
//
// When no value is stored at the path, or the stored value is not an int64,
// Int64 returns the given default, or 0 when none is given. Values of other
// numeric types, including int and [time.Duration], are not converted.
func (b Bag) Int64(
	path string,
	def ...int64,
) int64 {
	return bagGet(b, path, def)
}

// Uint returns the uint stored at the given path.
//
// When no value is stored at the path, or the stored value is not a uint,
// Uint returns the given default, or 0 when none is given. Values of other
// numeric types, including non-negative signed integers, are not converted.
func (b Bag) Uint(
	path string,
	def ...uint,
) uint {
	return bagGet(b, path, def)
}

// Uint8 returns the uint8 stored at the given path.
//
// When no value is stored at the path, or the stored value is not a uint8,
// Uint8 returns the given default, or 0 when none is given. Values of other
// numeric types are not converted.
//
// Since byte is an alias of uint8, a byte stored in the bag is returned by
// Uint8.
func (b Bag) Uint8(
	path string,
	def ...uint8,
) uint8 {
	return bagGet(b, path, def)
}

// Uint16 returns the uint16 stored at the given path.
//
// When no value is stored at the path, or the stored value is not a uint16,
// Uint16 returns the given default, or 0 when none is given. Values of
// other numeric types are not converted.
func (b Bag) Uint16(
	path string,
	def ...uint16,
) uint16 {
	return bagGet(b, path, def)
}

// Uint32 returns the uint32 stored at the given path.
//
// When no value is stored at the path, or the stored value is not a uint32,
// Uint32 returns the given default, or 0 when none is given. Values of
// other numeric types are not converted.
func (b Bag) Uint32(
	path string,
	def ...uint32,
) uint32 {
	return bagGet(b, path, def)
}

// Uint64 returns the uint64 stored at the given path.
//
// When no value is stored at the path, or the stored value is not a uint64,
// Uint64 returns the given default, or 0 when none is given. Values of
// other numeric types are not converted.
func (b Bag) Uint64(
	path string,
	def ...uint64,
) uint64 {
	return bagGet(b, path, def)
}

// Float32 returns the float32 stored at the given path.
//
// When no value is stored at the path, or the stored value is not a
// float32, Float32 returns the given default, or 0 when none is given.
// Values of other numeric types, including float64, are not converted.
func (b Bag) Float32(
	path string,
	def ...float32,
) float32 {
	return bagGet(b, path, def)
}

// Float64 returns the float64 stored at the given path.
//
// When no value is stored at the path, or the stored value is not a
// float64, Float64 returns the given default, or 0 when none is given.
// Values of other numeric types, including float32 and the integer types,
// are not converted.
//
// Float64 is the getter to use for numbers decoded by encoding/json into a
// map[string]any, which are always float64.
func (b Bag) Float64(
	path string,
	def ...float64,
) float64 {
	return bagGet(b, path, def)
}

// Rune returns the rune stored at the given path.
//
// When no value is stored at the path, or the stored value is not a rune,
// Rune returns the given default, or 0 when none is given. A string, even
// one holding a single character, is not converted.
//
// Since rune is an alias of int32, any int32 stored in the bag is returned
// by Rune, and Rune is interchangeable with [Bag.Int32].
func (b Bag) Rune(
	path string,
	def ...rune,
) rune {
	return bagGet(b, path, def)
}

// String returns the string stored at the given path.
//
// When no value is stored at the path, or the stored value is not a string,
// String returns the given default, or the empty string when none is
// given. Values of other types, including []byte and types implementing
// [fmt.Stringer], are not converted.
//
// Despite its name, this method does not make Bag a [fmt.Stringer], since
// it takes a path: fmt never calls it and formats a Bag as a regular map.
func (b Bag) String(
	path string,
	def ...string,
) string {
	return bagGet(b, path, def)
}

// Slice returns the []any stored at the given path.
//
// A stored []any is returned as is, sharing its elements with the bag. A
// stored []string is converted into a new []any holding the same strings,
// and changes to the result do not affect the bag.
//
// When no value is stored at the path, or the stored value is of any other
// type, including other slice types such as []int, Slice returns the given
// default, or nil when none is given. A stored nil []any or nil []string is
// returned as nil, not replaced by the default.
func (b Bag) Slice(
	path string,
	def ...[]any,
) []any {
	v, ok := b.lookup(path)
	if !ok {
		return bagFirstDef(def)
	}

	switch t := v.(type) {
	case []any:
		return t
	case []string:
		if t == nil {
			return nil
		}
		result := make([]any, len(t))
		for i, s := range t {
			result[i] = s
		}
		return result
	}
	return bagFirstDef(def)
}

// StringSlice returns the []string stored at the given path.
//
// A stored []string is returned as is, sharing its elements with the bag. A
// stored []any whose elements are all strings, as produced by
// encoding/json, is converted into a new []string, and changes to the
// result do not affect the bag. An empty []any converts to an empty
// []string.
//
// When no value is stored at the path, the stored value is of any other
// type, or a stored []any holds at least one element that is not a string,
// StringSlice returns the given default, or nil when none is given. A
// stored nil []string or nil []any is returned as nil, not replaced by the
// default.
func (b Bag) StringSlice(
	path string,
	def ...[]string,
) []string {
	v, ok := b.lookup(path)
	if !ok {
		return bagFirstDef(def)
	}

	switch t := v.(type) {
	case []string:
		return t
	case []any:
		if t == nil {
			return nil
		}
		result := make([]string, len(t))
		for i, e := range t {
			s, ok := e.(string)
			if !ok {
				return bagFirstDef(def)
			}
			result[i] = s
		}
		return result
	}
	return bagFirstDef(def)
}

// Duration returns the [time.Duration] stored at the given path.
//
// When no value is stored at the path, or the stored value is not a
// time.Duration, Duration returns the given default, or 0 when none is
// given. Integers are not interpreted as nanoseconds, and strings such as
// "5s" are not parsed; use [time.ParseDuration] on the result of
// [Bag.String] for the latter.
func (b Bag) Duration(
	path string,
	def ...time.Duration,
) time.Duration {
	return bagGet(b, path, def)
}

// Bag returns the bag stored at the given path.
//
// Bag and map[string]any values are returned without copying: a stored
// map[string]any is returned as a Bag sharing the same underlying map, so
// changes made through the result, including with [Bag.Set],
// [Bag.Delete] and [Bag.Merge], are visible in the parent bag. The empty
// path returns the bag itself.
//
// The result is a [flam.IBag] holding a Bag. When no value is stored at the
// path, or the stored value is of any other type, Bag returns the given
// default, which may be any flam.IBag, or nil when none is given. A stored
// nil bag is returned as a nil flam.IBag, not replaced by the default, so
// that the result compares equal to nil.
//
//	db := cfg.Bag("database")
//	host := db.String("host", "localhost")
func (b Bag) Bag(
	path string,
	def ...flam.IBag,
) flam.IBag {
	if v, ok := b.lookup(path); ok {
		if bag, isBag := bagAs(v); isBag {
			if bag == nil {
				return nil
			}
			return bag
		}
	}
	return bagFirstDef(def)
}

// All returns an iterator over the top level entries of the bag, yielding
// each key with its value, in increasing key order, so that the iteration
// is deterministic, unlike ranging over the map. A nil or empty bag yields
// nothing. Nested bags are yielded as single values: use [Bag.Bag] or All
// on them to iterate over their own entries.
//
// The values are yielded as stored, without copying, as by [Bag.Get], so a
// yielded map or slice shares its contents with the bag.
//
// The keys are collected when the iteration starts: the loop may modify
// the bag, an entry deleted before being reached is skipped, its value
// being read when reached, and an entry added is not yielded.
//
//	for key, value := range b.All() {
//		fmt.Println(key, value)
//	}
func (b Bag) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for _, key := range slices.Sorted(maps.Keys(b)) {
			val, ok := b[key]
			if !ok {
				continue // deleted by the loop
			}
			if !yield(key, val) {
				return
			}
		}
	}
}

// Clone returns a deep copy of the bag.
//
// Nested Bag, map[string]any, []any and []string values are copied at any
// depth and keep their dynamic type, so the copy can be modified without
// affecting the original, and the other way around. Values of any other
// type are copied as Go assigns them: pointers, other slice and map types,
// channels, functions and values containing them remain shared between
// the original and the copy.
//
// The result is a [flam.IBag] holding a Bag. Clone returns a nil
// flam.IBag on a nil bag, so that the result compares equal to nil, and an
// empty bag on an empty one. The bag must not contain itself, as described
// in [Bag].
func (b Bag) Clone() flam.IBag {
	if b == nil {
		return nil
	}
	return bagCopy(b).(Bag)
}

// Set stores the value at the given path, replacing any value already
// stored there, whatever its type, including a whole nested bag.
//
// Missing intermediate bags are created along the path, as values of type
// Bag, and intermediate empty slots (nil values and nil bags) are replaced
// with new Bags. An existing intermediate map[string]any is used, and
// modified, in place.
//
// The value is stored as is, without copying: a map or slice remains shared
// with the caller, and later changes to it are visible in the bag. Pass the
// result of [Bag.Clone] to store an independent copy of a bag.
//
// Set returns an [*flam.Error] wrapping:
//   - [ErrInvalidPath] when the path is empty or malformed, with the
//     "path" context field holding the path;
//   - [ErrNil] when the bag is nil, with the "path" context field
//     holding the path;
//   - [ErrPathConflict] when an intermediate segment holds a value that
//     is not a bag, with the "path" context field holding the path and the
//     "segment" field holding the path up to the conflicting value.
//
// When an error is returned, the bag is left unchanged.
//
//	b := container.Bag{}
//	_ = b.Set("server.port", 8080)    // b = {server: {port: 8080}}
//	err := b.Set("server.port.x", 1)  // ErrPathConflict, segment "server.port"
func (b Bag) Set(
	path string,
	val any,
) error {
	parts, ok := splitBagPath(path)
	if !ok || len(parts) == 0 {
		return newErrInvalidPath(bagSetMessage, path)
	}

	if b == nil {
		return newErrNil(bagSetMessage, path)
	}

	parents := parts[:len(parts)-1]
	if i := b.conflict(parents); i >= 0 {
		return newErrPathConflict(bagSetMessage, path).
			With(ErrorFieldSegment, strings.Join(parts[:i+1], BagPathSeparator))
	}

	b.ensure(parents)[parts[len(parts)-1]] = val
	return nil
}

// Delete removes the value stored at the given path, whatever its type,
// including a whole nested bag.
//
// Deleting a value that does not exist, including one under a missing
// intermediate bag or under a segment holding a value that is not a bag,
// is not an error and leaves the bag unchanged. Bags left empty by the
// deletion are kept, not removed from their parent. Deleting from a nil
// bag is a no-op.
//
// Delete returns an [*flam.Error] wrapping [ErrInvalidPath], with the "path"
// context field holding the path, when the path is empty or malformed. The
// root bag itself cannot be deleted.
func (b Bag) Delete(
	path string,
) error {
	parts, ok := splitBagPath(path)
	if !ok || len(parts) == 0 {
		return newErrInvalidPath(bagDeleteMessage, path)
	}

	parent, ok := b.lookupParts(parts[:len(parts)-1])
	if !ok {
		return nil
	}
	if bag, isBag := bagAs(parent); isBag {
		delete(bag, parts[len(parts)-1])
	}
	return nil
}

// Merge deep merges src into the bag stored at the given path, and returns
// the resulting bag.
//
// The empty path merges src into the bag itself. Otherwise, when no bag is
// stored at the path yet, one is created there first, along with any
// missing intermediate bags, as done by [Bag.Set].
//
// Keys of src, and of the bags nested in it, are interpreted as paths: a
// key such as "a.b" is expanded into the key "b" of a bag stored under the
// key "a", so that
//
//	b.Merge("", container.Bag{"server.port": 8080})
//	b.Merge("", container.Bag{"server": container.Bag{"port": 8080}})
//
// are equivalent. The keys of a bag are applied in sorted order, following
// the rules below, so when several keys define the same path, the result
// is the same on every run: with {"a": 1, "a.b": 2}, "a.b" comes last and
// "a" becomes a bag holding "b"; with {"a.b": 1, "a": {"b": 2}}, "a.b"
// comes last and "b" is 1.
//
// Only bags nested in bags are traversed: the bags held by a slice of src
// are copied as they are, their keys are not interpreted as paths, and a
// map[any]any they hold is not converted.
//
// A map[any]any in src, as produced by gopkg.in/yaml.v2, is converted into
// a Bag: its keys are converted with [fmt.Sprint], so the key 1 becomes
// "1", and are then interpreted as paths like any other key. Keys with the
// same form, such as 1 and "1", are applied in the order of their type
// name, then of their Go syntax, and the last one wins: the key of type
// string wins over the key of type int, which wins over float64.
//
// Values are merged key by key:
//   - when both the source and the destination values are bags (Bag or
//     map[string]any), they are merged recursively, following these same
//     rules;
//   - otherwise, the source value replaces the destination value, even a
//     whole destination bag, or is added when the key is missing.
//     Destination empty slots (nil values and nil bags) are replaced.
//     Slices are replaced, not appended to.
//
// As a consequence, merging never removes keys from the destination: keys
// missing from src are left untouched, and a nil or empty bag in src
// leaves an existing destination bag unchanged (it is only added when the
// key is missing). A nil or empty src adds no values, though the bag at the
// path is still created when missing.
//
// The values taken from src are copied, as done by [Bag.Clone], before the
// destination is modified, so src may share data with the destination, or
// even be the destination itself, and later changes to src do not affect
// the bag. Destination bags are modified in place and keep their dynamic
// type; bags taken from src keep their dynamic type, except map[any]any
// bags, which are converted to Bag.
//
// Merge modifies the receiver and returns it, except when the receiver is
// nil: a new bag is then created and returned, so the result must always be
// used, as with the built-in append:
//
//	var cfg flam.IBag = container.Bag(nil)
//	cfg, err = cfg.Merge("", defaults)
//	cfg, err = cfg.Merge("database", overrides)
//
// Merge returns the bag, unchanged, along with an [*flam.Error] wrapping:
//
//   - [ErrInvalidPath] when the path is malformed, with the "path"
//     context field holding the path;
//
//   - [ErrInvalidPath] when a key of src, or of a bag nested in it, is
//     empty or malformed, with the "path" context field holding the path
//     and the "key" field holding the culprit key, preceded for a nested
//     key by the keys leading to it from src, as described in
//     [ErrorFieldKey];
//
//   - [ErrPathConflict] when the path cannot be followed or created
//     because one of its segments, including the last one, holds a value
//     that is not a bag, with the "path" context field holding the path and
//     the "segment" field holding the path up to the conflicting value.
//
//   - [ErrUnsupportedBag] when src is a flam.IBag implementation other than
//     Bag whose Get("") does not return a Bag or a map[string]any, with the
//     "path" context field holding the path.
//
// These are checked in the order path, src and keys of src, the keys in
// sorted order, then the path conflicts, so the error returned is the same
// on every run.
//
// The result is a [flam.IBag] holding the Bag. src may be any flam.IBag: a
// nil one adds no values, and an implementation other than Bag is read
// through its Get("") method, which must return the whole bag as a Bag or a
// map[string]any.
func (b Bag) Merge(
	path string,
	src flam.IBag,
) (flam.IBag, error) {
	parts, ok := splitBagPath(path)
	if !ok {
		return b, newErrInvalidPath(bagMergeMessage, path)
	}

	source, ok := bagFromIBag(src)
	if !ok {
		return b, newErrUnsupportedBag(bagMergeMessage, path)
	}

	expanded, invalidKey, ok := bagExpand(source)
	if !ok {
		return b, newErrInvalidPath(bagMergeMessage, path).
			With(ErrorFieldKey, invalidKey)
	}

	if i := b.conflict(parts); i >= 0 {
		return b, newErrPathConflict(bagMergeMessage, path).
			With(ErrorFieldSegment, strings.Join(parts[:i+1], BagPathSeparator))
	}

	if b == nil {
		b = Bag{}
	}
	bagMerge(b.ensure(parts), expanded)
	return b, nil
}

// messages of the errors returned by the modifying methods
const (
	bagSetMessage    = "unable to set bag value"
	bagDeleteMessage = "unable to delete bag value"
	bagMergeMessage  = "unable to merge bag"
)

// bagFromIBag returns the content of a merged flam.IBag as a Bag: a nil
// flam.IBag, or one holding a nil pointer, is a nil Bag, a Bag is itself, and any other implementation is
// read through Get(""), which must return a Bag or a map[string]any.
func bagFromIBag(
	src flam.IBag,
) (Bag, bool) {
	if value.IsNil(src) {
		return nil, true
	}
	if t, ok := src.(Bag); ok {
		return t, true
	}
	return bagAs(src.Get(""))
}

// lookup returns the value stored at the given path and whether it exists.
// A malformed path, or one going through a value that is not a bag, does
// not exist; the empty path returns the bag itself. The path is walked
// segment by segment, without allocating.
func (b Bag) lookup(
	path string,
) (any, bool) {
	var current any = b
	for rest, more := path, path != ""; more; {
		var part string
		part, rest, more = strings.Cut(rest, BagPathSeparator)
		if part == "" {
			return nil, false
		}

		bag, isBag := bagAs(current)
		if !isBag {
			return nil, false
		}
		if current, isBag = bag[part]; !isBag {
			return nil, false
		}
	}
	return current, true
}

// lookupParts returns the value stored at the path made of the given
// segments and whether it exists. No segments returns the bag itself.
func (b Bag) lookupParts(
	parts []string,
) (any, bool) {
	var current any = b
	for _, part := range parts {
		bag, ok := bagAs(current)
		if !ok {
			return nil, false
		}
		if current, ok = bag[part]; !ok {
			return nil, false
		}
	}
	return current, true
}

// conflict returns the index of the first of the given segments holding a
// value that is not a bag, or -1 when a bag can be reached or created at
// the end of the segments. It does not modify the bag.
func (b Bag) conflict(
	parts []string,
) int {
	current := b
	for i, part := range parts {
		next, ok := current[part]
		if !ok || bagIsEmptySlot(next) {
			return -1
		}
		if current, ok = bagAs(next); !ok {
			return i
		}
	}
	return -1
}

// ensure returns the bag at the end of the given segments, creating missing
// levels and replacing empty slots with new Bags. It must only be called on
// a non-nil bag, once conflict reported no conflict for the same segments.
func (b Bag) ensure(
	parts []string,
) Bag {
	current := b
	for _, part := range parts {
		next, ok := current[part]
		if !ok || bagIsEmptySlot(next) {
			next = Bag{}
			current[part] = next
		}
		current, _ = bagAs(next)
	}
	return current
}

// bagGet returns the value stored at the given path when it exists and is
// exactly of type T, and the first default, or the zero value of T,
// otherwise.
func bagGet[T any](
	b Bag,
	path string,
	def []T,
) T {
	if v, ok := b.lookup(path); ok {
		if t, ok := v.(T); ok {
			return t
		}
	}
	return bagFirstDef(def)
}

// bagFirstDef returns the first of the given defaults, or the zero value of
// T when none is given.
func bagFirstDef[T any](
	values []T,
) T {
	if len(values) > 0 {
		return values[0]
	}
	var zero T
	return zero
}

// bagAs returns v as a Bag when it is a Bag or a map[string]any, sharing
// the same underlying map, and reports whether it was one of those.
func bagAs(
	v any,
) (Bag, bool) {
	switch t := v.(type) {
	case Bag:
		return t, true
	case map[string]any:
		return t, true
	}
	return nil, false
}

// bagIsEmptySlot reports whether v is nil or a nil bag.
func bagIsEmptySlot(
	v any,
) bool {
	bag, isBag := bagAs(v)
	return v == nil || (isBag && bag == nil)
}

// ValidPath reports whether the path is valid in a [Bag]: the empty path,
// addressing the bag itself, or a path with no empty segment, unlike
// "a..b", ".a" or "a.".
func ValidPath(
	path string,
) bool {
	_, ok := splitBagPath(path)
	return ok
}

// splitBagPath splits a path into its segments. The empty path addresses
// the bag itself and yields no segments; a path with an empty segment
// (ex: "a..b", ".a", "a.") is invalid.
func splitBagPath(
	path string,
) ([]string, bool) {
	if path == "" {
		return nil, true
	}

	parts := strings.Split(path, BagPathSeparator)
	if slices.Contains(parts, "") {
		return nil, false
	}
	return parts, true
}

// bagExpand returns a new bag holding a copy of src in which dotted keys
// are expanded into nested bags and map[any]any bags are converted to Bag,
// as described in Bag.Merge. It reports the first invalid key, in sorted
// order, when there is one. A nil src expands to a nil bag.
func bagExpand(
	src Bag,
) (Bag, string, bool) {
	if src == nil {
		return nil, "", true
	}

	result := Bag{}
	for _, key := range slices.Sorted(maps.Keys(src)) {
		parts, ok := splitBagPath(key)
		if !ok || len(parts) == 0 {
			return nil, key, false
		}

		val, invalidKey, ok := bagExpandValue(src[key])
		if !ok {
			return nil, key + BagPathSeparator + invalidKey, false
		}

		// wrap the value into the bags named by the key, then merge it
		for i := len(parts) - 1; i > 0; i-- {
			val = Bag{parts[i]: val}
		}
		bagMerge(result, Bag{parts[0]: val})
	}
	return result, "", true
}

// bagExpandValue returns the expanded copy of a value of a merged bag:
// non-nil bags are expanded, keeping the map[string]any type, map[any]any
// bags are converted then expanded and other values are copied.
func bagExpandValue(
	val any,
) (any, string, bool) {
	switch t := val.(type) {
	case Bag:
		if t != nil {
			return bagExpand(t)
		}
	case map[string]any:
		if t != nil {
			expanded, invalidKey, ok := bagExpand(t)
			return map[string]any(expanded), invalidKey, ok
		}
	case map[any]any:
		if t == nil {
			return Bag(nil), "", true
		}
		return bagExpand(bagFromAnyMap(t))
	}
	return bagCopy(val), "", true
}

// bagFromAnyMap returns m as a Bag keyed by the fmt.Sprint form of its keys.
// Keys sharing a form are applied in the order of their type name, then of
// their Go syntax, so the last one wins on every run. Values are not
// copied.
func bagFromAnyMap(
	m map[any]any,
) Bag {
	// the entries are collected with their values, as a lookup misses a
	// NaN key, and their sort forms formatted once
	type entry struct {
		typ, syntax, key string
		value            any
	}
	entries := make([]entry, 0, len(m))
	for k, v := range m {
		entries = append(entries, entry{
			typ: fmt.Sprintf("%T", k), syntax: fmt.Sprintf("%#v", k), key: fmt.Sprint(k), value: v,
		})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if c := strings.Compare(a.typ, b.typ); c != 0 {
			return c
		}
		return strings.Compare(a.syntax, b.syntax)
	})

	result := make(Bag, len(m))
	for _, e := range entries {
		result[e.key] = e.value
	}
	return result
}

// bagMerge merges src into dst, as described in Bag.Merge: bags present on
// both sides are merged recursively and any other src value replaces the
// dst one. The values of src are moved into dst without being copied.
func bagMerge(
	dst Bag,
	src Bag,
) {
	for k, v := range src {
		srcBag, srcIsBag := bagAs(v)
		dstBag, dstIsBag := bagAs(dst[k])
		if srcIsBag && dstIsBag && dstBag != nil {
			bagMerge(dstBag, srcBag)
			continue
		}
		dst[k] = v
	}
}

// bagCopy returns a deep copy of v when it is a Bag, a map[string]any, a
// []any or a []string, keeping its dynamic type and nil-ness, and v itself
// otherwise. v must not contain itself.
func bagCopy(
	v any,
) any {
	switch t := v.(type) {
	case Bag:
		if t == nil {
			return t
		}
		result := make(Bag, len(t))
		for k, e := range t {
			result[k] = bagCopy(e)
		}
		return result
	case map[string]any:
		return map[string]any(bagCopy(Bag(t)).(Bag))
	case []any:
		if t == nil {
			return t
		}
		result := make([]any, len(t))
		for i, e := range t {
			result[i] = bagCopy(e)
		}
		return result
	case []string:
		return slices.Clone(t)
	}
	return v
}
