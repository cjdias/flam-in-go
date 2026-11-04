package flam

import (
	"iter"
	"time"
)

// IBag is a tree of loosely typed values addressed by dot separated paths,
// for code that reads or changes loosely structured data without depending
// on a concrete type, such as a component taking its configuration as an
// IBag that tests can fake.
//
// The container.Bag type implements IBag, and its documentation describes
// in full the paths, defaults, errors and copies of each method. Methods
// taking or returning a bag use IBag, so that the interface does not depend
// on its implementation.
type IBag interface {
	// Has reports whether a value is stored at the path.
	Has(path string) bool

	// Get returns the value stored at the path, whatever its type, or the
	// first default.
	Get(path string, def ...any) any

	// Bool returns the bool stored at the path, or the first default.
	Bool(path string, def ...bool) bool

	// Int returns the int stored at the path, or the first default.
	Int(path string, def ...int) int

	// Int8 returns the int8 stored at the path, or the first default.
	Int8(path string, def ...int8) int8

	// Int16 returns the int16 stored at the path, or the first default.
	Int16(path string, def ...int16) int16

	// Int32 returns the int32 stored at the path, or the first default.
	Int32(path string, def ...int32) int32

	// Int64 returns the int64 stored at the path, or the first default.
	Int64(path string, def ...int64) int64

	// Uint returns the uint stored at the path, or the first default.
	Uint(path string, def ...uint) uint

	// Uint8 returns the uint8 stored at the path, or the first default.
	Uint8(path string, def ...uint8) uint8

	// Uint16 returns the uint16 stored at the path, or the first default.
	Uint16(path string, def ...uint16) uint16

	// Uint32 returns the uint32 stored at the path, or the first default.
	Uint32(path string, def ...uint32) uint32

	// Uint64 returns the uint64 stored at the path, or the first default.
	Uint64(path string, def ...uint64) uint64

	// Float32 returns the float32 stored at the path, or the first default.
	Float32(path string, def ...float32) float32

	// Float64 returns the float64 stored at the path, or the first default.
	Float64(path string, def ...float64) float64

	// Rune returns the rune stored at the path, or the first default.
	Rune(path string, def ...rune) rune

	// String returns the string stored at the path, or the first default.
	String(path string, def ...string) string

	// Slice returns the []any stored at the path, or the first default.
	Slice(path string, def ...[]any) []any

	// StringSlice returns the []string stored at the path, or the first
	// default.
	StringSlice(path string, def ...[]string) []string

	// Duration returns the time.Duration stored at the path, or the first
	// default.
	Duration(path string, def ...time.Duration) time.Duration

	// Bag returns the bag stored at the path, or the first default.
	Bag(path string, def ...IBag) IBag

	// All returns an iterator over the top level entries of the bag, keys
	// and values, in key order.
	All() iter.Seq2[string, any]

	// Clone returns a deep copy of the bag.
	Clone() IBag

	// Set stores the value at the path, creating the missing intermediate
	// bags.
	Set(path string, value any) error

	// Delete removes the value stored at the path.
	Delete(path string) error

	// Merge deep merges src into the bag stored at the path, and returns the
	// resulting bag, which must be used.
	Merge(path string, src IBag) (IBag, error)
}
