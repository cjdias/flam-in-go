// Package testutil holds the assertions shared by the tests of the flam
// packages.
package testutil

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Equal fails the test when got and want are not deeply equal, as
// reflect.DeepEqual tells, which distinguishes nil from empty containers
// and compares dynamic types.
func Equal(t testing.TB, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", name, got, want)
	}
}

// NoError fails the test, and stops it, when err is not nil.
func NoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

// Error fails the test, and stops it, unless err is a *flam.Error
// wrapping sentinel, with the given text and exactly the given context.
func Error(t testing.TB, err, sentinel error, text string, ctx map[string]any) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %v", sentinel)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T, want a *flam.Error", err)
	}
	if err.Error() != text {
		t.Errorf("Error() = %q, want %q", err.Error(), text)
	}
	Context(t, e, ctx)
}

// Context fails the test unless e holds exactly the given context, which
// must be a non-nil map.
func Context(t testing.TB, e *flam.Error, want map[string]any) {
	t.Helper()
	got := e.Context()
	if got == nil {
		t.Fatalf("Context() = nil, want a non-nil map")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Context() = %#v, want %#v", got, want)
	}
}

// Panics fails the test when fn does not panic.
func Panics(t testing.TB, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", name)
		}
	}()
	fn()
}

// NotPanics fails the test, and stops it, when fn panics.
func NotPanics(t testing.TB, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", name, r)
		}
	}()
	fn()
}

// SamePointer reports whether a and b are maps, slices or pointers sharing
// the same underlying data, which tells a value from its copy.
func SamePointer(a, b any) bool {
	return reflect.ValueOf(a).UnsafePointer() == reflect.ValueOf(b).UnsafePointer()
}
