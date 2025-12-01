package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_Bag_Clone(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		expected flam.Bag
	}{
		{
			test:     "should clone an empty bag",
			bag:      flam.Bag{},
			expected: flam.Bag{}},
		{
			test:     "should clone a simple bag",
			bag:      flam.Bag{"field": 123},
			expected: flam.Bag{"field": 123}},
		{
			test:     "should clone a nested bag",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 456}},
			expected: flam.Bag{"field": flam.Bag{"subfield": 456}}},
		{
			test:     "should clone a nested bag (references)",
			bag:      flam.Bag{"field": &flam.Bag{"subfield": 456}},
			expected: flam.Bag{"field": flam.Bag{"subfield": 456}}},
		{
			test:     "should clone a bag with an array",
			bag:      flam.Bag{"field": []any{1, 2, 3}},
			expected: flam.Bag{"field": []any{1, 2, 3}}},
		{
			test:     "should clone a bag with a nested array",
			bag:      flam.Bag{"field": []any{1, flam.Bag{"subfield": 2}, 3}},
			expected: flam.Bag{"field": []any{1, flam.Bag{"subfield": 2}, 3}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			result := scenario.bag.Clone()
			require.NotNil(t, result)
			require.NotSame(t, &result, &scenario.bag)
			assert.Equal(t, scenario.expected, result)
		})
	}
}

func Test_Bag_Entries(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		expected []string
	}{
		{
			test:     "should return nil for an empty bag",
			bag:      flam.Bag{},
			expected: nil},
		{
			test:     "should return a list of entries for a bag with one entry",
			bag:      flam.Bag{"field": 123},
			expected: []string{"field"}},
		{
			test:     "should return a list of entries for a bag with multiple entries",
			bag:      flam.Bag{"field1": 123, "field2": 456},
			expected: []string{"field1", "field2"}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.ElementsMatch(t, scenario.expected, scenario.bag.Entries())
		})
	}
}

func Test_Bag_Has(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		expected bool
	}{
		{
			test:     "should return true for an empty path",
			bag:      flam.Bag{},
			path:     "",
			expected: true},
		{
			test:     "should return false when checking for a path in an empty bag",
			bag:      flam.Bag{},
			path:     "field",
			expected: false},
		{
			test:     "should return true for a matching path",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			expected: true},
		{
			test:     "should return false for a non-matching path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			expected: false},
		{
			test:     "should return true for a matching nested path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 456}},
			path:     "field.subfield",
			expected: true},
		{
			test:     "should return false for a non-matching nested path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 456}},
			path:     "field.nonexistent",
			expected: false},
		{
			test:     "should return true for a path with consecutive dots",
			bag:      flam.Bag{"a": flam.Bag{"b": 1}},
			path:     "a..b",
			expected: true},
		{
			test:     "should return true for a path with leading dots",
			bag:      flam.Bag{"a": 1},
			path:     ".a",
			expected: true},
		{
			test:     "should return true for a path with trailing dots",
			bag:      flam.Bag{"a": 1},
			path:     "a.",
			expected: true},
		{
			test:     "should return false for a path through a non-bag value",
			bag:      flam.Bag{"a": 1},
			path:     "a.b",
			expected: false},
		{
			test:     "should return true for a path through a pointer to a bag",
			bag:      flam.Bag{"a": &flam.Bag{"b": 1}},
			path:     "a.b",
			expected: true},
		{
			test:     "should return false for a non-matching path through a pointer to a bag",
			bag:      flam.Bag{"a": &flam.Bag{"b": 1}},
			path:     "a.c",
			expected: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Has(scenario.path))
		})
	}
}

func Test_Bag_Get(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []any
		expected any
	}{
		{
			test:     "should return an empty bag for an empty path on an empty bag",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: flam.Bag{}},
		{
			test:     "should return the bag itself for an empty path",
			bag:      flam.Bag{"field": 123},
			path:     "",
			def:      nil,
			expected: flam.Bag{"field": 123}},
		{
			test:     "should return a value for a valid path",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: 123},
		{
			test:     "should return a nested value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 456}},
			path:     "field.subfield",
			def:      nil,
			expected: 456},
		{
			test:     "should return nil for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: nil},
		{
			test:     "should return the bag itself for an empty path even with a default value",
			bag:      flam.Bag{"field": 123},
			path:     "",
			def:      []any{"default"},
			expected: flam.Bag{"field": 123}},
		{
			test:     "should return a value for a valid path even with a default value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      []any{"default"},
			expected: 123},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []any{"default"},
			expected: "default"},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 456}},
			path:     "field.nonexistent",
			def:      []any{"default"},
			expected: "default"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Get(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Bool(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []bool
		expected bool
	}{
		{
			test:     "should return false for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: false},
		{
			test:     "should return the bool value for a valid path",
			bag:      flam.Bag{"field": true},
			path:     "field",
			def:      nil,
			expected: true},
		{
			test:     "should return false for a valid path with a non-bool value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: false},
		{
			test:     "should return the nested bool value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": true}},
			path:     "field.subfield",
			def:      nil,
			expected: true},
		{
			test:     "should return false for an invalid path without a default value",
			bag:      flam.Bag{"field": true},
			path:     "nonexistent",
			def:      nil,
			expected: false},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []bool{true},
			expected: true},
		{
			test:     "should return the bool value for a valid path when a default is provided",
			bag:      flam.Bag{"field": true},
			path:     "field",
			def:      []bool{false},
			expected: true},
		{
			test:     "should return the default value for a valid path with a non-bool value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []bool{true},
			expected: true},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": true},
			path:     "nonexistent",
			def:      []bool{true},
			expected: true},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": true}},
			path:     "field.nonexistent",
			def:      []bool{true},
			expected: true},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Bool(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Int(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []int
		expected int
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: 123},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123}},
			path:     "field.subfield",
			def:      nil,
			expected: 123},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []int{456},
			expected: 456},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      []int{456},
			expected: 123},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []int{456},
			expected: 456},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []int{456},
			expected: 456},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123}},
			path:     "field.nonexistent",
			def:      []int{456},
			expected: 456},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Int(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Int8(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []int8
		expected int8
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": int8(123)},
			path:     "field",
			def:      nil,
			expected: int8(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": int8(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: int8(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []int8{56},
			expected: 56},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": int8(123)},
			path:     "field",
			def:      []int8{56},
			expected: int8(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []int8{56},
			expected: 56},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []int8{56},
			expected: 56},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123}},
			path:     "field.nonexistent",
			def:      []int8{56},
			expected: 56},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Int8(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Int16(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []int16
		expected int16
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": int16(123)},
			path:     "field",
			def:      nil,
			expected: int16(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": int16(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: int16(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []int16{56},
			expected: int16(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": int16(123)},
			path:     "field",
			def:      []int16{56},
			expected: int16(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []int16{56},
			expected: int16(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []int16{56},
			expected: int16(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123}},
			path:     "field.nonexistent",
			def:      []int16{56},
			expected: int16(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Int16(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Int32(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []int32
		expected int32
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": int32(123)},
			path:     "field",
			def:      nil,
			expected: int32(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": int32(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: int32(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []int32{56},
			expected: int32(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": int32(123)},
			path:     "field",
			def:      []int32{56},
			expected: int32(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []int32{56},
			expected: int32(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []int32{56},
			expected: int32(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123}},
			path:     "field.nonexistent",
			def:      []int32{56},
			expected: int32(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Int32(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Int64(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []int64
		expected int64
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": int64(123)},
			path:     "field",
			def:      nil,
			expected: int64(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": int64(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: int64(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []int64{56},
			expected: int64(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": int64(123)},
			path:     "field",
			def:      []int64{56},
			expected: int64(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []int64{56},
			expected: int64(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []int64{56},
			expected: int64(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123}},
			path:     "field.nonexistent",
			def:      []int64{56},
			expected: int64(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Int64(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Uint(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []uint
		expected uint
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": uint(123)},
			path:     "field",
			def:      nil,
			expected: uint(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: uint(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []uint{56},
			expected: uint(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": uint(123)},
			path:     "field",
			def:      []uint{56},
			expected: uint(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []uint{56},
			expected: uint(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []uint{56},
			expected: uint(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint(123)}},
			path:     "field.nonexistent",
			def:      []uint{56},
			expected: uint(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Uint(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Uint8(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []uint8
		expected uint8
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": uint8(123)},
			path:     "field",
			def:      nil,
			expected: uint8(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint8(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: uint8(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []uint8{56},
			expected: uint8(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": uint8(123)},
			path:     "field",
			def:      []uint8{56},
			expected: uint8(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []uint8{56},
			expected: uint8(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []uint8{56},
			expected: uint8(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint8(123)}},
			path:     "field.nonexistent",
			def:      []uint8{56},
			expected: uint8(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Uint8(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Uint16(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []uint16
		expected uint16
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": uint16(123)},
			path:     "field",
			def:      nil,
			expected: uint16(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint16(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: uint16(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []uint16{56},
			expected: uint16(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": uint16(123)},
			path:     "field",
			def:      []uint16{56},
			expected: uint16(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []uint16{56},
			expected: uint16(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []uint16{56},
			expected: uint16(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint16(123)}},
			path:     "field.nonexistent",
			def:      []uint16{56},
			expected: uint16(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Uint16(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Uint32(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []uint32
		expected uint32
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": uint32(123)},
			path:     "field",
			def:      nil,
			expected: uint32(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint32(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: uint32(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []uint32{56},
			expected: uint32(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": uint32(123)},
			path:     "field",
			def:      []uint32{56},
			expected: uint32(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []uint32{56},
			expected: uint32(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []uint32{56},
			expected: uint32(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint32(123)}},
			path:     "field.nonexistent",
			def:      []uint32{56},
			expected: uint32(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Uint32(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Uint64(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []uint64
		expected uint64
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the int value for a valid path",
			bag:      flam.Bag{"field": uint64(123)},
			path:     "field",
			def:      nil,
			expected: uint64(123)},
		{
			test:     "should return 0 for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the nested int value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint64(123)}},
			path:     "field.subfield",
			def:      nil,
			expected: uint64(123)},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []uint64{56},
			expected: uint64(56)},
		{
			test:     "should return the int value for a valid path when a default is provided",
			bag:      flam.Bag{"field": uint64(123)},
			path:     "field",
			def:      []uint64{56},
			expected: uint64(123)},
		{
			test:     "should return the default value for a valid path with a non-int value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []uint64{56},
			expected: uint64(56)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123},
			path:     "nonexistent",
			def:      []uint64{56},
			expected: uint64(56)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": uint64(123)}},
			path:     "field.nonexistent",
			def:      []uint64{56},
			expected: uint64(56)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Uint64(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Float32(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []float32
		expected float32
	}{
		{
			test:     "should return 0.0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: float32(0.0)},
		{
			test:     "should return the float value for a valid path",
			bag:      flam.Bag{"field": float32(123.45)},
			path:     "field",
			def:      nil,
			expected: float32(123.45)},
		{
			test:     "should return 0.0 for a valid path with a non-float value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: float32(0.0)},
		{
			test:     "should return the nested float value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": float32(123.45)}},
			path:     "field.subfield",
			def:      nil,
			expected: float32(123.45)},
		{
			test:     "should return 0.0 for an invalid path without a default value",
			bag:      flam.Bag{"field": float32(123.45)},
			path:     "nonexistent",
			def:      nil,
			expected: float32(0.0)},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []float32{456.78},
			expected: float32(456.78)},
		{
			test:     "should return the float value for a valid path when a default is provided",
			bag:      flam.Bag{"field": float32(123.45)},
			path:     "field",
			def:      []float32{456.78},
			expected: float32(123.45)},
		{
			test:     "should return the default value for a valid path with a non-float value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []float32{456.78},
			expected: float32(456.78)},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": float32(123.45)},
			path:     "nonexistent",
			def:      []float32{456.78},
			expected: float32(456.78)},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": float32(123.45)}},
			path:     "field.nonexistent",
			def:      []float32{456.78},
			expected: float32(456.78)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Float32(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Float64(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []float64
		expected float64
	}{
		{
			test:     "should return 0.0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0.0},
		{
			test:     "should return the float value for a valid path",
			bag:      flam.Bag{"field": 123.45},
			path:     "field",
			def:      nil,
			expected: 123.45},
		{
			test:     "should return 0.0 for a valid path with a non-float value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0.0},
		{
			test:     "should return the nested float value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123.45}},
			path:     "field.subfield",
			def:      nil,
			expected: 123.45},
		{
			test:     "should return 0.0 for an invalid path without a default value",
			bag:      flam.Bag{"field": 123.45},
			path:     "nonexistent",
			def:      nil,
			expected: 0.0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []float64{456.78},
			expected: 456.78},
		{
			test:     "should return the float value for a valid path when a default is provided",
			bag:      flam.Bag{"field": 123.45},
			path:     "field",
			def:      []float64{456.78},
			expected: 123.45},
		{
			test:     "should return the default value for a valid path with a non-float value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []float64{456.78},
			expected: 456.78},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": 123.45},
			path:     "nonexistent",
			def:      []float64{456.78},
			expected: 456.78},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": 123.45}},
			path:     "field.nonexistent",
			def:      []float64{456.78},
			expected: 456.78},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Float64(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_String(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []string
		expected string
	}{
		{
			test:     "should return empty string for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: ""},
		{
			test:     "should return the string value for a valid path",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: "value"},
		{
			test:     "should return empty string for a valid path with a non-string value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: ""},
		{
			test:     "should return the nested string value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.subfield",
			def:      nil,
			expected: "value"},
		{
			test:     "should return empty string for an invalid path without a default value",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      nil,
			expected: ""},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []string{"default"},
			expected: "default"},
		{
			test:     "should return the string value for a valid path when a default is provided",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []string{"default"},
			expected: "value"},
		{
			test:     "should return the default value for a valid path with a non-string value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      []string{"default"},
			expected: "default"},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      []string{"default"},
			expected: "default"},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.nonexistent",
			def:      []string{"default"},
			expected: "default"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.String(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_StringMap(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []map[string]any
		expected map[string]any
	}{
		{
			test:     "should return nil for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: nil},
		{
			test:     "should return the string map value for a valid path",
			bag:      flam.Bag{"field": map[string]any{"a": 1}},
			path:     "field",
			def:      nil,
			expected: map[string]any{"a": 1}},
		{
			test:     "should return nil for a valid path with a non-string value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: nil},
		{
			test:     "should return the nested string map value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": map[string]any{"a": 1}}},
			path:     "field.subfield",
			def:      nil,
			expected: map[string]any{"a": 1}},
		{
			test:     "should return nil for an invalid path without a default value",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      nil,
			expected: nil},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []map[string]any{{"default": 1}},
			expected: map[string]any{"default": 1}},
		{
			test:     "should return the string map value for a valid path when a default is provided",
			bag:      flam.Bag{"field": map[string]any{"a": 1}},
			path:     "field",
			def:      []map[string]any{{"default": 1}},
			expected: map[string]any{"a": 1}},
		{
			test:     "should return the default value for a valid path with a non-string map value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      []map[string]any{{"default": 1}},
			expected: map[string]any{"default": 1}},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      []map[string]any{{"default": 1}},
			expected: map[string]any{"default": 1}},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.nonexistent",
			def:      []map[string]any{{"default": 1}},
			expected: map[string]any{"default": 1}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.StringMap(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_StringMapString(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []map[string]string
		expected map[string]string
	}{
		{
			test:     "should return nil for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: nil},
		{
			test:     "should return the string map value for a valid path",
			bag:      flam.Bag{"field": map[string]string{"a": "1"}},
			path:     "field",
			def:      nil,
			expected: map[string]string{"a": "1"}},
		{
			test:     "should return nil for a valid path with a non-string value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: nil},
		{
			test:     "should return the nested string map value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": map[string]string{"a": "1"}}},
			path:     "field.subfield",
			def:      nil,
			expected: map[string]string{"a": "1"}},
		{
			test:     "should return nil for an invalid path without a default value",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      nil,
			expected: nil},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []map[string]string{{"default": "1"}},
			expected: map[string]string{"default": "1"}},
		{
			test:     "should return the string map value for a valid path when a default is provided",
			bag:      flam.Bag{"field": map[string]string{"a": "1"}},
			path:     "field",
			def:      []map[string]string{{"default": "1"}},
			expected: map[string]string{"a": "1"}},
		{
			test:     "should return the default value for a valid path with a non-string map value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      []map[string]string{{"default": "1"}},
			expected: map[string]string{"default": "1"}},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      []map[string]string{{"default": "1"}},
			expected: map[string]string{"default": "1"}},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.nonexistent",
			def:      []map[string]string{{"default": "1"}},
			expected: map[string]string{"default": "1"}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.StringMapString(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Slice(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      [][]any
		expected []any
	}{
		{
			test:     "should return nil for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: nil},
		{
			test:     "should return the slice value for a valid path",
			bag:      flam.Bag{"field": []any{"a", 1}},
			path:     "field",
			def:      nil,
			expected: []any{"a", 1}},
		{
			test:     "should return nil for a valid path with a non-string value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: nil},
		{
			test:     "should return the nested slice value for a valid path",
			bag:      flam.Bag{"field": []any{"a", 1}},
			path:     "field",
			def:      nil,
			expected: []any{"a", 1}},
		{
			test:     "should return nil for an invalid path without a default value",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      nil,
			expected: nil},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      [][]any{{1}},
			expected: []any{1}},
		{
			test:     "should return the slice value for a valid path when a default is provided",
			bag:      flam.Bag{"field": []any{"a", 1}},
			path:     "field",
			def:      [][]any{{1}},
			expected: []any{"a", 1}},
		{
			test:     "should return the default value for a valid path with a non-string map value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      [][]any{{1}},
			expected: []any{1}},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      [][]any{{1}},
			expected: []any{1}},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.nonexistent",
			def:      [][]any{{1}},
			expected: []any{1}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Slice(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_StringSlice(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      [][]string
		expected []string
	}{
		{
			test:     "should return nil for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: nil},
		{
			test:     "should return the string slice value for a valid path",
			bag:      flam.Bag{"field": []string{"a", "b"}},
			path:     "field",
			def:      nil,
			expected: []string{"a", "b"}},
		{
			test:     "should return nil for a valid path with a non-slice value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: nil},
		{
			test:     "should return nil for a valid path with a slice of non-string",
			bag:      flam.Bag{"field": []int{1, 2}},
			path:     "field",
			def:      nil,
			expected: nil},
		{
			test:     "should return the nested string slice value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": []string{"a", "b"}}},
			path:     "field.subfield",
			def:      nil,
			expected: []string{"a", "b"}},
		{
			test:     "should return nil for an invalid path without a default value",
			bag:      flam.Bag{"field": []string{"a", "b"}},
			path:     "nonexistent",
			def:      nil,
			expected: nil},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      [][]string{{"c", "d"}},
			expected: []string{"c", "d"}},
		{
			test:     "should return the string slice value for a valid path when a default is provided",
			bag:      flam.Bag{"field": []string{"a", "b"}},
			path:     "field",
			def:      [][]string{{"c", "d"}},
			expected: []string{"a", "b"}},
		{
			test:     "should return the default value for a valid path with a non-slice value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      [][]string{{"c", "d"}},
			expected: []string{"c", "d"}},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": []string{"a", "b"}},
			path:     "nonexistent",
			def:      [][]string{{"c", "d"}},
			expected: []string{"c", "d"}},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": []string{"a", "b"}}},
			path:     "field.nonexistent",
			def:      [][]string{{"c", "d"}},
			expected: []string{"c", "d"}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.StringSlice(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Duration(t *testing.T) {
	defaultValue := time.Minute

	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []time.Duration
		expected time.Duration
	}{
		{
			test:     "should return 0 for an empty path without a default value",
			bag:      flam.Bag{},
			path:     "",
			def:      nil,
			expected: 0},
		{
			test:     "should return the duration value for a valid path",
			bag:      flam.Bag{"field": time.Second},
			path:     "field",
			def:      nil,
			expected: time.Second},
		{
			test:     "should return 0 for a valid path with a non-duration value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      nil,
			expected: 0},
		{
			test:     "should return the duration value for a valid path",
			bag:      flam.Bag{"field": time.Second},
			path:     "field",
			def:      nil,
			expected: time.Second},
		{
			test:     "should return the duration value for a valid path (int conversion)",
			bag:      flam.Bag{"field": 1000},
			path:     "field",
			def:      nil,
			expected: 1000 * time.Millisecond},
		{
			test:     "should return the duration value for a valid path (int64 conversion)",
			bag:      flam.Bag{"field": int64(1000)},
			path:     "field",
			def:      nil,
			expected: 1000 * time.Millisecond},
		{
			test:     "should return 0 for an invalid path without a default value",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      nil,
			expected: 0},
		{
			test:     "should return the default value for an empty path",
			bag:      flam.Bag{},
			path:     "",
			def:      []time.Duration{defaultValue},
			expected: defaultValue},
		{
			test:     "should return the duration value for a valid path when a default is provided",
			bag:      flam.Bag{"field": time.Second},
			path:     "field",
			def:      []time.Duration{defaultValue},
			expected: time.Second},
		{
			test:     "should return the default value for a valid path with a non-duration value",
			bag:      flam.Bag{"field": "value"},
			path:     "field",
			def:      []time.Duration{defaultValue},
			expected: defaultValue},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      []time.Duration{defaultValue},
			expected: defaultValue},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.nonexistent",
			def:      []time.Duration{defaultValue},
			expected: defaultValue},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Duration(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Bag(t *testing.T) {
	scenarios := []struct {
		test     string
		bag      flam.Bag
		path     string
		def      []flam.Bag
		expected flam.Bag
	}{
		{
			test:     "should return a copy for an empty path without a default value",
			bag:      flam.Bag{"field": 123},
			path:     "",
			def:      nil,
			expected: flam.Bag{"field": 123}},
		{
			test:     "should return the bag value for a valid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field",
			def:      nil,
			expected: flam.Bag{"subfield": "value"}},
		{
			test:     "should return nil for a valid path with a non-bag value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      nil,
			expected: nil},
		{
			test:     "should return nil for an invalid path without a default value",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      nil,
			expected: nil},
		{
			test:     "should return the bag value for a valid path when a default is provided",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field",
			def:      []flam.Bag{{"default": "value"}},
			expected: flam.Bag{"subfield": "value"}},
		{
			test:     "should return the default value for a valid path with a non-bag value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			def:      []flam.Bag{{"default": "value"}},
			expected: flam.Bag{"default": "value"}},
		{
			test:     "should return the default value for an invalid path",
			bag:      flam.Bag{"field": "value"},
			path:     "nonexistent",
			def:      []flam.Bag{{"default": "value"}},
			expected: flam.Bag{"default": "value"}},
		{
			test:     "should return the default value for a deep invalid path",
			bag:      flam.Bag{"field": flam.Bag{"subfield": "value"}},
			path:     "field.nonexistent",
			def:      []flam.Bag{{"default": "value"}},
			expected: flam.Bag{"default": "value"}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			assert.Equal(t, scenario.expected, scenario.bag.Bag(scenario.path, scenario.def...))
		})
	}
}

func Test_Bag_Set(t *testing.T) {
	scenarios := []struct {
		test        string
		bag         flam.Bag
		path        string
		value       any
		expected    flam.Bag
		expectedErr error
	}{
		{
			test:     "should set a value at the top level",
			bag:      flam.Bag{},
			path:     "field",
			value:    123,
			expected: flam.Bag{"field": 123}},
		{
			test:     "should set a value in a nested bag",
			bag:      flam.Bag{"nested": flam.Bag{}},
			path:     "nested.field",
			value:    "value",
			expected: flam.Bag{"nested": flam.Bag{"field": "value"}}},
		{
			test:     "should set a value in a nested bag (dot sequence)",
			bag:      flam.Bag{"nested": flam.Bag{}},
			path:     "nested...field",
			value:    "value",
			expected: flam.Bag{"nested": flam.Bag{"field": "value"}}},
		{
			test:     "should create nested bags to set a value",
			bag:      flam.Bag{},
			path:     "a.b.c",
			value:    true,
			expected: flam.Bag{"a": flam.Bag{"b": flam.Bag{"c": true}}}},
		{
			test:     "should overwrite an existing value",
			bag:      flam.Bag{"field": 123},
			path:     "field",
			value:    "new_value",
			expected: flam.Bag{"field": "new_value"}},
		{
			test:        "should return an error for an empty path",
			bag:         flam.Bag{},
			path:        "",
			value:       123,
			expectedErr: flam.ErrBagInvalidPath},
		{
			test:     "should overwrite a non-bag value with a bag",
			bag:      flam.Bag{"a": 123},
			path:     "a.b",
			value:    "hello",
			expected: flam.Bag{"a": flam.Bag{"b": "hello"}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			e := scenario.bag.Set(scenario.path, scenario.value)

			if scenario.expectedErr != nil {
				assert.ErrorIs(t, e, scenario.expectedErr)
				return
			}

			assert.NoError(t, e)
			assert.Equal(t, scenario.expected, scenario.bag)
		})
	}
}

func Test_Bag_Merge(t *testing.T) {
	scenarios := []struct {
		test     string
		dest     flam.Bag
		src      flam.Bag
		expected flam.Bag
	}{
		{
			test:     "should merge simple values into an empty bag",
			dest:     flam.Bag{},
			src:      flam.Bag{"a": 1, "b": "hello"},
			expected: flam.Bag{"a": 1, "b": "hello"}},
		{
			test:     "should add new values without overwriting existing ones",
			dest:     flam.Bag{"a": 1},
			src:      flam.Bag{"b": 2},
			expected: flam.Bag{"a": 1, "b": 2}},
		{
			test:     "should overwrite existing values",
			dest:     flam.Bag{"a": 1, "b": "old"},
			src:      flam.Bag{"b": "new", "c": 3},
			expected: flam.Bag{"a": 1, "b": "new", "c": 3}},
		{
			test:     "should merge nested bags",
			dest:     flam.Bag{"nested": flam.Bag{"a": 1}},
			src:      flam.Bag{"nested": flam.Bag{"b": 2}},
			expected: flam.Bag{"nested": flam.Bag{"a": 1, "b": 2}}},
		{
			test:     "should merge nested bags (pointer in src)",
			dest:     flam.Bag{"nested": flam.Bag{"a": 1}},
			src:      flam.Bag{"nested": &flam.Bag{"b": 2}},
			expected: flam.Bag{"nested": flam.Bag{"a": 1, "b": 2}}},
		{
			test:     "should create nested bag if destination has a non-bag value",
			dest:     flam.Bag{"a": 123},
			src:      flam.Bag{"a": flam.Bag{"b": "hello"}},
			expected: flam.Bag{"a": flam.Bag{"b": "hello"}}},
		{
			test:     "should handle complex nested merging",
			dest:     flam.Bag{"a": flam.Bag{"b": 1}, "c": "foo"},
			src:      flam.Bag{"a": flam.Bag{"d": 2}, "c": "bar", "e": flam.Bag{"f": 3}},
			expected: flam.Bag{"a": flam.Bag{"b": 1, "d": 2}, "c": "bar", "e": flam.Bag{"f": 3}}},
		{
			test:     "should merge into a nested pointer bag",
			dest:     flam.Bag{"nested": &flam.Bag{"a": 1}},
			src:      flam.Bag{"nested": flam.Bag{"b": 2}},
			expected: flam.Bag{"nested": &flam.Bag{"a": 1, "b": 2}}},
		{
			test:     "should merge pointer bag into a nested pointer bag",
			dest:     flam.Bag{"nested": &flam.Bag{"a": 1}},
			src:      flam.Bag{"nested": &flam.Bag{"b": 2}},
			expected: flam.Bag{"nested": &flam.Bag{"a": 1, "b": 2}}},
		{
			test:     "should create nested bag from pointer if destination has a non-bag value",
			dest:     flam.Bag{"a": 123},
			src:      flam.Bag{"a": &flam.Bag{"b": "hello"}},
			expected: flam.Bag{"a": flam.Bag{"b": "hello"}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.test, func(t *testing.T) {
			scenario.dest.Merge(scenario.src)
			assert.Equal(t, scenario.expected, scenario.dest)
		})
	}
}

func Test_Bag_Populate(t *testing.T) {
	type simpleStruct struct {
		Field int
	}

	type complexStruct struct {
		Name     string `mapstructure:"name"`
		Value    int    `mapstructure:"value"`
		Nested   simpleStruct
		Children []string
	}

	t.Run("without path", func(t *testing.T) {
		scenarios := []struct {
			test        string
			bag         flam.Bag
			target      any
			expected    any
			expectedErr error
		}{
			{
				test:     "should populate a struct with a simple scalar value",
				bag:      flam.Bag{"field": 123},
				target:   &simpleStruct{},
				expected: &simpleStruct{Field: 123}},
			{
				test: "should populate a complex struct with tags",
				bag: flam.Bag{
					"name":  "test_name",
					"value": 999,
					"Nested": flam.Bag{
						"Field": 789},
					"Children": []any{"child1", "child2"}},
				target: &complexStruct{},
				expected: &complexStruct{
					Name:     "test_name",
					Value:    999,
					Nested:   simpleStruct{Field: 789},
					Children: []string{"child1", "child2"}},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.test, func(t *testing.T) {
				e := scenario.bag.Populate(scenario.target)

				if scenario.expectedErr != nil {
					assert.ErrorIs(t, e, scenario.expectedErr)
					return
				}

				assert.NoError(t, e)
				assert.Equal(t, scenario.expected, scenario.target)
			})
		}
	})

	t.Run("with path", func(t *testing.T) {
		scenarios := []struct {
			test        string
			bag         flam.Bag
			path        string
			target      any
			expected    any
			expectedErr error
		}{
			{
				test:     "should populate a struct from a nested path",
				bag:      flam.Bag{"config": flam.Bag{"field": 456}},
				path:     "config",
				target:   &simpleStruct{},
				expected: &simpleStruct{Field: 456}},
			{
				test:        "should return an error for an invalid path",
				bag:         flam.Bag{"config": flam.Bag{"field": 456}},
				path:        "invalid.path",
				target:      &simpleStruct{},
				expectedErr: flam.ErrBagInvalidPath},
			{
				test: "should populate a complex struct from a nested path",
				bag: flam.Bag{
					"data": flam.Bag{
						"name":  "nested_name",
						"value": 111,
						"Nested": flam.Bag{
							"Field": 222},
						"Children": []any{"c1"}}},
				path:   "data",
				target: &complexStruct{},
				expected: &complexStruct{
					Name:     "nested_name",
					Value:    111,
					Nested:   simpleStruct{Field: 222},
					Children: []string{"c1"},
				},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.test, func(t *testing.T) {
				e := scenario.bag.Populate(scenario.target, scenario.path)

				if scenario.expectedErr != nil {
					assert.ErrorIs(t, e, scenario.expectedErr)
					return
				}

				assert.NoError(t, e)
				assert.Equal(t, scenario.expected, scenario.target)
			})
		}
	})
}

func Test_BagNormalization(t *testing.T) {
	scenarios := []struct {
		name string
		val  any
		want any
	}{
		{
			name: "Bag",
			val:  flam.Bag{"KEY": "value"},
			want: flam.Bag{"key": "value"}},
		{
			name: "slice of any",
			val:  []any{flam.Bag{"KEY": "value"}},
			want: []any{flam.Bag{"key": "value"}}},
		{
			name: "map[string]any",
			val:  map[string]any{"KEY": "value"},
			want: flam.Bag{"key": "value"}},
		{
			name: "map[any]any with string key",
			val:  map[any]any{"KEY": "value"},
			want: flam.Bag{"key": "value"}},
		{
			name: "map[any]any with non-string key",
			val:  map[any]any{123: "value"},
			want: flam.Bag{"123": "value"}},
		{
			name: "float64 convertible to int",
			val:  123.0,
			want: 123},
		{
			name: "float64 not convertible to int",
			val:  123.4,
			want: 123.4},
		{
			name: "other primitive types",
			val:  "a string",
			want: "a string"},
		{
			name: "nested structure",
			val:  flam.Bag{"L1": map[any]any{"L2": []any{1.0, "VALUE"}}},
			want: flam.Bag{"l1": flam.Bag{"l2": []any{1, "VALUE"}}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			assert.Equal(t, scenario.want, flam.BagNormalization(scenario.val))
		})
	}
}
