package flam

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// compile time check that *Error satisfies the error interface
var _ error = (*flam.Error)(nil)

// ---------------------------------------------------------------------------
// error kinds used to exercise the nil detection of wrapped causes
// ---------------------------------------------------------------------------

type ptrError struct{ msg string }

func (e *ptrError) Error() string { return e.msg } // panics on a nil receiver

type valueError struct{ msg string }

func (e valueError) Error() string { return e.msg }

type stringError string

func (e stringError) Error() string { return string(e) }

type mapError map[string]string

func (e mapError) Error() string { return "map error" }

type sliceError []string

func (e sliceError) Error() string { return "slice error" }

type funcError func() string

func (e funcError) Error() string { return e() } // panics on a nil func

type chanError chan struct{}

func (e chanError) Error() string { return "chan error" }

// asError stores e in an error interface, as a function returning error
// would.
func asError(e *flam.Error) error { return e }

// types based on the copied containers, for the copy tests
type (
	namedMap    map[string]any
	namedSlice  []any
	namedString []string
	stringKey   string
)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func Test_NewError(t *testing.T) {
	t.Run("should return a new error using the message as text", func(t *testing.T) {
		e := flam.NewError("message")
		if e == nil {
			t.Fatal("NewError() = nil")
		}
		testutil.Equal(t, "Error()", e.Error(), "message")
	})

	t.Run("should keep the message verbatim", func(t *testing.T) {
		for _, msg := range []string{"", " ", "  padded  ", "a: b", "line\nbreak", "ünïcødé ✓", "%s %d %v"} {
			testutil.Equal(t, "Error()", flam.NewError(msg).Error(), msg)
		}
	})

	t.Run("should have no cause", func(t *testing.T) {
		e := flam.NewError("message")
		if e.Unwrap() != nil || errors.Unwrap(e) != nil {
			t.Error("NewError() has a cause")
		}
	})

	t.Run("should have an empty non-nil context without maps", func(t *testing.T) {
		testutil.Context(t, flam.NewError("message"), map[string]any{})
		testutil.Context(t, flam.NewError("message", nil, map[string]any{}, nil), map[string]any{})
	})

	t.Run("should merge the context maps, the last value winning", func(t *testing.T) {
		e := flam.NewError("message",
			map[string]any{"a": 1, "b": 1},
			nil,
			map[string]any{"b": 2, "c": 2},
			map[string]any{"c": 3})
		testutil.Context(t, e, map[string]any{"a": 1, "b": 2, "c": 3})
	})

	t.Run("should keep nil values and the empty key", func(t *testing.T) {
		testutil.Context(t, flam.NewError("message", map[string]any{"nil": nil, "": 1}),
			map[string]any{"nil": nil, "": 1})
	})

	t.Run("should not be affected by later changes to the given maps", func(t *testing.T) {
		ctx := map[string]any{"a": 1}
		e := flam.NewError("message", ctx)
		ctx["a"] = 2
		ctx["b"] = 3
		testutil.Context(t, e, map[string]any{"a": 1})
	})

	t.Run("should return a distinct error on every call", func(t *testing.T) {
		a, b := flam.NewError("message"), flam.NewError("message")
		if a == b || errors.Is(a, b) {
			t.Error("two NewError() errors are the same or match each other")
		}
		if !errors.Is(a, a) {
			t.Error("an error does not match itself")
		}
	})

	t.Run("should not match another error with the same message and context", func(t *testing.T) {
		ctx := map[string]any{"k": "v"}
		a, b := flam.NewError("message", ctx), flam.NewError("message", ctx)
		if errors.Is(a, b) || errors.Is(b, a) {
			t.Error("two equal NewError() errors match each other")
		}
		var target *flam.Error
		if !errors.As(a, &target) || target != a {
			t.Error("errors.As() did not find the *Error itself")
		}
	})

	t.Run("should keep a key whose value is replaced by nil in a later map", func(t *testing.T) {
		testutil.Context(t, flam.NewError("message", map[string]any{"k": 1}, map[string]any{"k": nil}),
			map[string]any{"k": nil})
	})
}

// ---------------------------------------------------------------------------
// context copies
// ---------------------------------------------------------------------------

func Test_ContextCopy(t *testing.T) {
	t.Run("should deep copy string keyed maps and slices of any or string", func(t *testing.T) {
		nestedMap := map[string]any{"k": "v"}
		bag := container.Bag{"k": container.Bag{"deep": "v"}}
		named := namedMap{"k": "v"}
		anys := []any{"v", map[string]any{"k": "v"}}
		strs := []string{"v"}
		namedAnys := namedSlice{"v"}
		namedStrs := namedString{"v"}
		e := flam.NewError("message", map[string]any{
			"map": nestedMap, "bag": bag, "named": named, "anys": anys, "strs": strs,
			"namedAnys": namedAnys, "namedStrs": namedStrs,
		})

		nestedMap["k"] = "changed"
		bag["k"].(container.Bag)["deep"] = "changed"
		named["k"] = "changed"
		anys[0], anys[1].(map[string]any)["k"] = "changed", "changed"
		strs[0], namedAnys[0], namedStrs[0] = "changed", "changed", "changed"

		testutil.Context(t, e, map[string]any{
			"map": map[string]any{"k": "v"}, "bag": container.Bag{"k": container.Bag{"deep": "v"}},
			"named": namedMap{"k": "v"}, "anys": []any{"v", map[string]any{"k": "v"}}, "strs": []string{"v"},
			"namedAnys": namedSlice{"v"}, "namedStrs": namedString{"v"},
		})
	})

	t.Run("should keep the dynamic type of the copies", func(t *testing.T) {
		ctx := flam.NewError("message", map[string]any{
			"bag": container.Bag{}, "map": map[string]any{}, "named": namedMap{}, "namedAnys": namedSlice{},
		}).Context()
		for key, want := range map[string]string{
			"bag": "container.Bag", "map": "map[string]interface {}", "named": "flam.namedMap", "namedAnys": "flam.namedSlice",
		} {
			if got := fmt.Sprintf("%T", ctx[key]); got != want {
				t.Errorf("Context()[%q] has type %s, want %s", key, got, want)
			}
		}
	})

	t.Run("should keep nil containers nil and empty containers empty", func(t *testing.T) {
		values := map[string]any{
			"nilBag": container.Bag(nil), "nilMap": map[string]any(nil), "nilAnys": []any(nil), "nilStrs": []string(nil),
			"emptyBag": container.Bag{}, "emptyAnys": []any{}, "emptyStrs": []string{},
			"nilElems": []any{nil, map[string]any{"nil": nil}},
		}
		testutil.Context(t, flam.NewError("message", values), values) // DeepEqual tells nil from empty
	})

	t.Run("should copy every map with string keys and interface values", func(t *testing.T) {
		stringers := map[string]fmt.Stringer{"k": nil}
		errs := map[string]error{"k": errors.New("v")}
		e := flam.NewError("message", map[string]any{"stringers": stringers, "errs": errs})
		errs["k"] = errors.New("changed")
		if got := e.Context()["errs"].(map[string]error)["k"].Error(); got != "v" {
			t.Errorf("errs[k] = %q, want the original", got)
		}
		if got := e.Context()["stringers"].(map[string]fmt.Stringer); got == nil || len(got) != 1 || got["k"] != nil {
			t.Errorf("stringers = %#v, want a copy keeping the nil element", got)
		}
	})

	t.Run("should share every other value", func(t *testing.T) {
		ptr := &struct{ n int }{n: 1}
		intKeys := map[int]any{1: "v"}
		stringValues := map[string]string{"k": "v"}
		ints := []int{1}
		nested := [][]any{{"v"}}
		maps := []map[string]any{{"k": "v"}}
		ctx := flam.NewError("message", map[string]any{
			"ptr": ptr, "intKeys": intKeys, "stringValues": stringValues, "ints": ints, "nested": nested, "maps": maps,
		}).Context()
		if ctx["ptr"] != any(ptr) || !testutil.SamePointer(ctx["intKeys"], intKeys) ||
			!testutil.SamePointer(ctx["stringValues"], stringValues) || !testutil.SamePointer(ctx["ints"], ints) ||
			!testutil.SamePointer(ctx["nested"], nested) || !testutil.SamePointer(ctx["maps"], maps) {
			t.Error("a value of a type that is not copied was copied")
		}
	})

	t.Run("should deep copy containers nested at any depth", func(t *testing.T) {
		deepest := []string{"v"}
		inner := map[string]any{"strs": deepest}
		list := []any{inner}
		top := container.Bag{"list": list}
		e := flam.NewError("message", map[string]any{"top": top})

		deepest[0] = "changed"
		inner["added"] = 1
		top["added"] = 1
		list[0] = "replaced"

		testutil.Context(t, e, map[string]any{
			"top": container.Bag{"list": []any{map[string]any{"strs": []string{"v"}}}},
		})
	})

	t.Run("should keep the type and nil-ness of nested nil containers", func(t *testing.T) {
		values := map[string]any{
			"anys": []any{map[string]any(nil), container.Bag(nil), []any(nil), []string(nil), (*int)(nil), nil},
			"map":  map[string]any{"bag": container.Bag(nil), "nilMap": map[string]any(nil), "nil": nil},
		}
		testutil.Context(t, flam.NewError("message", values), values)
	})

	t.Run("should copy slices of interface elements other than any, keeping nil elements", func(t *testing.T) {
		errs := []error{errors.New("v"), nil}
		stringers := []fmt.Stringer{nil}
		e := flam.NewError("message", map[string]any{"errs": errs, "stringers": stringers})
		errs[0] = errors.New("changed")

		ctx := e.Context()
		gotErrs := ctx["errs"].([]error)
		if testutil.SamePointer(gotErrs, errs) || len(gotErrs) != 2 || gotErrs[0].Error() != "v" || gotErrs[1] != nil {
			t.Errorf("errs = %#v, want an independent copy keeping the nil element", gotErrs)
		}
		if got := ctx["stringers"].([]fmt.Stringer); testutil.SamePointer(got, stringers) || len(got) != 1 || got[0] != nil {
			t.Errorf("stringers = %#v, want an independent copy keeping the nil element", got)
		}
	})

	t.Run("should copy maps whose keys are of a type based on string", func(t *testing.T) {
		m := map[stringKey]any{"k": "v"}
		e := flam.NewError("message", map[string]any{"m": m})
		m["k"] = "changed"
		testutil.Context(t, e, map[string]any{"m": map[stringKey]any{"k": "v"}})
	})

	t.Run("should store arrays and structs as is, sharing the containers they hold", func(t *testing.T) {
		array := [1]map[string]any{{"k": "v"}}
		holder := struct{ M map[string]any }{M: map[string]any{"k": "v"}}
		e := flam.NewError("message", map[string]any{"array": array, "struct": holder})
		array[0]["k"] = "changed"
		holder.M["k"] = "changed"

		ctx := e.Context()
		testutil.Equal(t, "array", ctx["array"].([1]map[string]any)[0]["k"], "changed")
		testutil.Equal(t, "struct", ctx["struct"].(struct{ M map[string]any }).M["k"], "changed")
	})

	t.Run("should copy a container referenced twice into two independent copies", func(t *testing.T) {
		shared := map[string]any{"k": "v"}
		ctx := flam.NewError("message", map[string]any{"a": shared, "b": shared}).Context()
		if testutil.SamePointer(ctx["a"], ctx["b"]) || testutil.SamePointer(ctx["a"], shared) {
			t.Error("the copies are not independent maps")
		}
	})
}

// ---------------------------------------------------------------------------
// NewWrapped
// ---------------------------------------------------------------------------

func Test_NewWrappedError(t *testing.T) {
	t.Run("should compose the text from the message and the cause", func(t *testing.T) {
		tests := []struct {
			name    string
			cause   error
			message string
			want    string
		}{
			{"message and cause", errors.New("cause"), "message", "message: cause"},
			{"empty message", errors.New("cause"), "", "cause"},
			{"cause with empty text", errors.New(""), "message", "message: "},
			{"both empty", errors.New(""), "", ""},
			{"nil cause", nil, "message", "message"},
			{"nil cause and empty message", nil, "", ""},
			{"nested errors", flam.NewWrappedError(flam.NewError("c"), "b"), "a", "a: b: c"},
			{"fmt wrapped cause", fmt.Errorf("b: %w", errors.New("c")), "a", "a: b: c"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				testutil.Equal(t, "Error()", flam.NewWrappedError(tc.cause, tc.message).Error(), tc.want)
			})
		}
	})

	t.Run("should unwrap to the cause", func(t *testing.T) {
		cause := errors.New("cause")
		e := flam.NewWrappedError(cause, "message")
		if e.Unwrap() != cause || errors.Unwrap(e) != cause {
			t.Error("Unwrap() did not return the cause")
		}
		if flam.NewWrappedError(nil, "message").Unwrap() != nil {
			t.Error("Unwrap() of a nil cause is not nil")
		}
	})

	t.Run("should treat nil values in a non-nil error interface as no cause", func(t *testing.T) {
		for name, cause := range map[string]error{
			"nil *Error": (*flam.Error)(nil), "nil pointer": (*ptrError)(nil), "nil map": mapError(nil),
			"nil slice": sliceError(nil), "nil func": funcError(nil), "nil chan": chanError(nil),
		} {
			t.Run(name, func(t *testing.T) {
				var e *flam.Error
				testutil.NotPanics(t, "NewWrapped()", func() { e = flam.NewWrappedError(cause, "message") })
				testutil.NotPanics(t, "Error()", func() { testutil.Equal(t, "Error()", e.Error(), "message") })
				if got := e.Unwrap(); got != nil {
					t.Errorf("Unwrap() = %#v, want a nil interface", got)
				}
			})
		}
	})

	t.Run("should keep causes that are non-nil values", func(t *testing.T) {
		tests := map[string]struct {
			cause error
			want  string
		}{
			"pointer":             {&ptrError{msg: "ptr"}, "message: ptr"},
			"struct value":        {valueError{msg: "value"}, "message: value"},
			"zero struct value":   {valueError{}, "message: "},
			"string value":        {stringError("string"), "message: string"},
			"empty non-nil map":   {mapError{}, "message: map error"},
			"empty non-nil slice": {sliceError{}, "message: slice error"},
			"non-nil func":        {funcError(func() string { return "func" }), "message: func"},
			"non-nil chan":        {make(chanError), "message: chan error"},
		}
		for name, tc := range tests {
			e := flam.NewWrappedError(tc.cause, "message")
			testutil.Equal(t, name+" Error()", e.Error(), tc.want)
			if e.Unwrap() == nil {
				t.Errorf("%s: Unwrap() = nil, want the cause", name)
			}
		}
	})

	t.Run("should be matched by errors.Is and errors.As through the chain", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		inner := &ptrError{msg: "inner"}
		e := flam.NewWrappedError(fmt.Errorf("middle: %w", flam.NewWrappedError(sentinel, "inner")), "outer")
		if !errors.Is(e, sentinel) || errors.Is(e, errors.New("sentinel")) {
			t.Error("errors.Is() does not match exactly the wrapped sentinel")
		}
		var target *ptrError
		if !errors.As(flam.NewWrappedError(fmt.Errorf("m: %w", inner), "outer"), &target) || target != inner {
			t.Error("errors.As() did not find the wrapped error")
		}
		var flamTarget *flam.Error
		if !errors.As(fmt.Errorf("wrapped: %w", e), &flamTarget) || flamTarget != e {
			t.Error("errors.As() did not find the *Error")
		}
	})

	t.Run("should copy the context, without inheriting the cause one", func(t *testing.T) {
		cause := flam.NewError("cause", map[string]any{"inner": 1})
		ctx := map[string]any{"outer": map[string]any{"k": "v"}}
		e := flam.NewWrappedError(cause, "message", ctx, map[string]any{"b": 2})
		ctx["outer"].(map[string]any)["k"] = "changed"
		testutil.Context(t, e, map[string]any{"outer": map[string]any{"k": "v"}, "b": 2})
		testutil.Context(t, cause, map[string]any{"inner": 1})
	})

	t.Run("should merge the context maps like NewError, the last value winning", func(t *testing.T) {
		e := flam.NewWrappedError(errors.New("cause"), "message",
			map[string]any{"a": 1, "b": 1}, nil, map[string]any{}, map[string]any{"b": 2, "": nil})
		testutil.Context(t, e, map[string]any{"a": 1, "b": 2, "": nil})
		testutil.Context(t, flam.NewWrappedError(errors.New("cause"), "message"), map[string]any{})
	})

	t.Run("should have an empty context with a nil cause and no maps", func(t *testing.T) {
		testutil.Context(t, flam.NewWrappedError(nil, "message"), map[string]any{})
	})

	t.Run("should return a distinct error on every call, even with the same cause", func(t *testing.T) {
		cause := errors.New("cause")
		a, b := flam.NewWrappedError(cause, "message"), flam.NewWrappedError(cause, "message")
		if a == b || errors.Is(a, b) || errors.Is(b, a) {
			t.Error("two NewWrappedError() errors are the same or match each other")
		}
		if !errors.Is(a, cause) || !errors.Is(b, cause) {
			t.Error("the errors do not match their shared cause")
		}
	})

	t.Run("should match a wrapped *Error cause by identity only", func(t *testing.T) {
		inner := flam.NewError("inner", map[string]any{"k": "v"})
		outer := flam.NewWrappedError(inner, "outer")
		if !errors.Is(outer, inner) {
			t.Error("errors.Is(outer, inner) = false, want true")
		}
		if errors.Is(outer, flam.NewError("inner", map[string]any{"k": "v"})) {
			t.Error("errors.Is() matched an equal but distinct *Error")
		}
	})

	t.Run("should render the text of a chain, skipping empty messages", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
			want string
		}{
			{"empty middle message", flam.NewWrappedError(flam.NewWrappedError(errors.New("c"), ""), "a"), "a: c"},
			{"empty *Error cause", flam.NewWrappedError(flam.NewError(""), "a"), "a: "},
			{"empty messages throughout", flam.NewWrappedError(flam.NewWrappedError(flam.NewError("c"), ""), ""), "c"},
			{"cause message with a separator", flam.NewWrappedError(errors.New("b: c"), "a"), "a: b: c"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				testutil.Equal(t, "Error()", tc.err.Error(), tc.want)
			})
		}
	})

	t.Run("should let the contexts of a chain be collected by walking it", func(t *testing.T) {
		inner := flam.NewError("inner", map[string]any{"inner": 1, "shared": "inner"})
		middle := fmt.Errorf("middle: %w", inner)
		outer := flam.NewWrappedError(middle, "outer", map[string]any{"outer": 2, "shared": "outer"})

		var contexts []map[string]any
		for err := error(outer); err != nil; err = errors.Unwrap(err) {
			if e, ok := err.(*flam.Error); ok {
				contexts = append(contexts, e.Context())
			}
		}
		testutil.Equal(t, "contexts", contexts, []map[string]any{
			{"outer": 2, "shared": "outer"},
			{"inner": 1, "shared": "inner"},
		})
	})
}

// ---------------------------------------------------------------------------
// Error, Unwrap and Context on nil and zero values
// ---------------------------------------------------------------------------

func Test_Error_NilAndZero(t *testing.T) {
	var nilErr *flam.Error
	zero := &flam.Error{}

	for name, e := range map[string]*flam.Error{"nil receiver": nilErr, "zero value": zero} {
		t.Run(name, func(t *testing.T) {
			testutil.NotPanics(t, "methods", func() {
				testutil.Equal(t, "Error()", e.Error(), "")
				if e.Unwrap() != nil {
					t.Error("Unwrap() is not nil")
				}
				ctx := e.Context()
				if ctx == nil || len(ctx) != 0 {
					t.Fatalf("Context() = %#v, want an empty non-nil map", ctx)
				}
				ctx["k"] = "v" // writable
			})
		})
	}

	t.Run("should derive from a zero value with With", func(t *testing.T) {
		e := zero.With("k", "v")
		testutil.Equal(t, "Error()", e.Error(), "")
		testutil.Context(t, e, map[string]any{"k": "v"})
		if e.Unwrap() != nil {
			t.Error("Unwrap() of an error derived from a zero value is not nil")
		}
		testutil.Context(t, zero, map[string]any{})
	})

	t.Run("should be usable through a nil *Error stored in an error interface", func(t *testing.T) {
		err := asError(nilErr)
		testutil.NotPanics(t, "methods", func() {
			testutil.Equal(t, "Error()", err.Error(), "")
			if errors.Unwrap(err) != nil {
				t.Error("errors.Unwrap() is not nil")
			}
		})
	})

	t.Run("should behave the same through a copy of the value", func(t *testing.T) {
		cause := errors.New("cause")
		e := flam.NewWrappedError(cause, "message", map[string]any{"k": "v"})
		c := *e
		testutil.Equal(t, "Error()", c.Error(), e.Error())
		if c.Unwrap() != cause {
			t.Error("Unwrap() of the copy did not return the cause")
		}
		testutil.Context(t, &c, map[string]any{"k": "v"})
		_ = c.With("k", "changed")
		testutil.Context(t, e, map[string]any{"k": "v"})
	})
}

func Test_Error_Text(t *testing.T) {
	e := flam.NewWrappedError(errors.New("cause"), "message", map[string]any{"key": "value"})
	testutil.Equal(t, "Error()", e.Error(), "message: cause") // the context is not in the text
	testutil.Equal(t, "%v", fmt.Sprintf("%v", e), "message: cause")
	testutil.Equal(t, "%s", fmt.Sprintf("%s", e), "message: cause")
	testutil.Equal(t, "%q", fmt.Sprintf("%q", e), `"message: cause"`)

	inner := errors.New("inner")
	middle := flam.NewWrappedError(inner, "middle")
	if flam.NewWrappedError(middle, "outer").Unwrap() != middle {
		t.Error("Unwrap() did not go back a single level")
	}
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

func Test_Error_Context(t *testing.T) {
	newError := func() *flam.Error {
		return flam.NewError("message", map[string]any{
			"map":     map[string]any{"k": "v"},
			"bag":     container.Bag{"k": container.Bag{"deep": "v"}},
			"slice":   []any{"v", []any{"deep"}},
			"strings": []string{"v"},
		})
	}
	want := map[string]any{
		"map":     map[string]any{"k": "v"},
		"bag":     container.Bag{"k": container.Bag{"deep": "v"}},
		"slice":   []any{"v", []any{"deep"}},
		"strings": []string{"v"},
	}

	t.Run("should return a new copy on every call", func(t *testing.T) {
		e := newError()
		if testutil.SamePointer(e.Context(), e.Context()) {
			t.Error("Context() returned the same map twice")
		}
	})

	t.Run("should not expose the stored context, at any depth", func(t *testing.T) {
		e := newError()
		ctx := e.Context()
		ctx["added"] = 1
		delete(ctx, "map")
		ctx = e.Context()
		ctx["map"].(map[string]any)["k"] = "changed"
		ctx["bag"].(container.Bag)["k"].(container.Bag)["deep"] = "changed"
		ctx["slice"].([]any)[0] = "changed"
		ctx["slice"].([]any)[1].([]any)[0] = "changed"
		ctx["strings"].([]string)[0] = "changed"
		testutil.Context(t, e, want)
	})

	t.Run("should share pointers", func(t *testing.T) {
		ptr := &struct{}{}
		if flam.NewError("m", map[string]any{"p": ptr}).Context()["p"] != any(ptr) {
			t.Error("Context() copied a pointer")
		}
	})
}

// ---------------------------------------------------------------------------
// With
// ---------------------------------------------------------------------------

func Test_Error_With(t *testing.T) {
	t.Run("should return nil on a nil receiver, a non-nil error interface", func(t *testing.T) {
		var e *flam.Error
		got := e.With("k", "v")
		if got != nil {
			t.Fatalf("With() = %v, want nil", got)
		}
		if asError(got) == nil {
			t.Error("a nil *Error in an error interface compared equal to nil; update the With documentation")
		}
	})

	t.Run("should add or replace a field in a new error", func(t *testing.T) {
		base := flam.NewError("message", map[string]any{"a": 1})
		added := base.With("b", 2)
		replaced := base.With("a", 3)
		if added == base || replaced == base {
			t.Fatal("With() returned the receiver")
		}
		testutil.Context(t, added, map[string]any{"a": 1, "b": 2})
		testutil.Context(t, replaced, map[string]any{"a": 3})
		testutil.Context(t, base, map[string]any{"a": 1})
	})

	t.Run("should accept the empty field and a nil value, and chain", func(t *testing.T) {
		e := flam.NewError("message").With("", nil).With("a", 1).With("b", 2).With("a", 3)
		testutil.Context(t, e, map[string]any{"": nil, "a": 3, "b": 2})
	})

	t.Run("should keep the message and the cause, without wrapping the receiver", func(t *testing.T) {
		cause := errors.New("cause")
		base := flam.NewWrappedError(cause, "message")
		e := base.With("k", "v")
		testutil.Equal(t, "Error()", e.Error(), "message: cause")
		if e.Unwrap() != cause || !errors.Is(e, cause) {
			t.Error("With() lost the cause")
		}
		if errors.Is(e, base) {
			t.Error("errors.Is(derived, base) = true, want false")
		}
	})

	t.Run("should copy the value and keep derived errors independent", func(t *testing.T) {
		value := container.Bag{"k": "v"}
		base := flam.NewError("message", map[string]any{"shared": map[string]any{"k": "v"}})
		left, right := base.With("bag", value), base.With("side", "right")
		value["k"] = "changed"
		left.Context()["shared"].(map[string]any)["k"] = "changed"

		testutil.Context(t, left, map[string]any{"shared": map[string]any{"k": "v"}, "bag": container.Bag{"k": "v"}})
		testutil.Context(t, right, map[string]any{"shared": map[string]any{"k": "v"}, "side": "right"})
		testutil.Context(t, base, map[string]any{"shared": map[string]any{"k": "v"}})
	})

	t.Run("should copy the value at any depth, keeping its type and nil-ness", func(t *testing.T) {
		deep := []any{map[string]any{"k": []string{"v"}}}
		e := flam.NewError("message").
			With("deep", deep).
			With("nilBag", container.Bag(nil)).
			With("nilStrs", []string(nil)).
			With("named", namedMap{"k": "v"})
		deep[0].(map[string]any)["k"].([]string)[0] = "changed"
		testutil.Context(t, e, map[string]any{
			"deep": []any{map[string]any{"k": []string{"v"}}}, "nilBag": container.Bag(nil),
			"nilStrs": []string(nil), "named": namedMap{"k": "v"},
		})
	})

	t.Run("should share pointer values", func(t *testing.T) {
		ptr := &struct{ n int }{}
		if flam.NewError("message").With("p", ptr).Context()["p"] != any(ptr) {
			t.Error("With() copied a pointer")
		}
	})

	t.Run("should replace an existing field with nil, keeping the field", func(t *testing.T) {
		base := flam.NewError("message", map[string]any{"k": "v"})
		testutil.Context(t, base.With("k", nil), map[string]any{"k": nil})
		testutil.Context(t, base, map[string]any{"k": "v"})
	})

	t.Run("should keep having no cause when the receiver has none", func(t *testing.T) {
		e := flam.NewError("message").With("k", "v")
		if e.Unwrap() != nil {
			t.Error("Unwrap() of a derived error without cause is not nil")
		}
		testutil.Equal(t, "Error()", e.Error(), "message")
	})

	t.Run("should let errors.As find the shared cause, but not the receiver", func(t *testing.T) {
		inner := flam.NewError("inner")
		base := flam.NewWrappedError(inner, "outer")
		e := base.With("k", "v")
		var target *flam.Error
		if !errors.As(e, &target) || target != e {
			t.Error("errors.As() did not first find the derived error itself")
		}
		if !errors.As(errors.Unwrap(e), &target) || target != inner {
			t.Error("errors.As() did not find the shared cause")
		}
		if !errors.Is(e, inner) || errors.Is(e, base) {
			t.Error("errors.Is() did not match the cause only")
		}
	})
}

// ---------------------------------------------------------------------------
// concurrency
// ---------------------------------------------------------------------------

func Test_Error_ConcurrentUse(t *testing.T) {
	base := flam.NewWrappedError(errors.New("cause"), "message", map[string]any{"nested": map[string]any{"k": "v"}})
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			_ = base.Error()
			_ = base.Unwrap()
			base.Context()["nested"].(map[string]any)["k"] = i
			if base.With("worker", i).Context()["worker"] != i {
				t.Errorf("worker %d: With() lost its field", i)
			}
		})
	}
	wg.Wait()
	testutil.Context(t, base, map[string]any{"nested": map[string]any{"k": "v"}})
}
