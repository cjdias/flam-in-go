package flam

import (
	"maps"
	"reflect"

	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// Error is an error carrying a message, an optional cause and a set of
// key/value context fields describing the circumstances of the failure.
//
// An Error is immutable once created: no method modifies the receiver, and
// methods that "change" an error, such as [Error.With], return a new one.
// This makes an Error safe for concurrent use by multiple goroutines and
// safe to share as a base from which several more specific errors are
// derived.
//
// The context is kept separate from the error text: it is never included in
// the string returned by [Error.Error] and is retrieved with
// [Error.Context]. This lets the text stay stable and human readable while
// the context carries structured data meant for logs or for programmatic
// inspection.
//
// Errors should be created with [NewError] or [NewWrappedError]. The zero
// value, as in &Error{}, is nevertheless usable and behaves as an error with
// an empty message, no cause and an empty context. Every method also
// accepts a nil *Error receiver without panicking.
//
// An Error participates in the standard error chain through
// [Error.Unwrap], so [errors.Is] and [errors.As] see through it to its
// cause. An Error does not define an Is method: two distinct *Error values
// never match each other under [errors.Is], even when their message and
// context are equal. To test for a category of failure, wrap a sentinel
// error and match against the sentinel:
//
//	var ErrNotFound = errors.New("not found")
//
//	e := flam.NewWrappedError(ErrNotFound, "unable to load user").
//		With("id", id)
//
//	errors.Is(e, ErrNotFound) // true
type Error struct {
	message string
	cause   error
	context map[string]any
}

// NewError returns a new [Error] with the given message, no cause and a
// context built from the given maps.
//
// The message is stored verbatim and is used, as is, as the error text. An
// empty message is allowed and results in an empty error text.
//
// Any number of context maps may be given. They are merged in order into a
// single context, so when the same key is present in more than one map, the
// value from the last map wins. Nil and empty maps are ignored. Keys may be
// any string, including the empty string, and a key holding a nil value is
// kept.
//
// The context is copied, so changing the given maps after the call does not
// affect the returned error. The copy is deep, at any depth, for the maps
// with string keys and interface values, such as map[string]any or a type
// based on it (a container.Bag), and for the slices of interface or string
// elements, such as []any and []string; each copy keeps the dynamic type
// and the nil-ness of its original. Any other value, such as a pointer, a
// struct or a slice of another element type, is stored as is and remains
// shared with the caller. The values must not contain themselves, which
// would make the copy endless.
//
// Every call returns a new, distinct *Error.
//
//	e := flam.NewError("invalid configuration",
//		map[string]any{"file": path},
//		map[string]any{"line": line})
//
//	e.Error()   // "invalid configuration"
//	e.Context() // map[file:... line:...]
func NewError(
	message string,
	ctx ...map[string]any,
) *Error {
	return &Error{
		message: message,
		context: copyContext(ctx...),
	}
}

// NewWrappedError returns a new [Error] with the given message, wrapping the
// given cause, and with a context built from the given maps.
//
// The cause becomes the next error in the chain: it is returned by
// [Error.Unwrap] and is therefore visible to [errors.Is] and [errors.As]. It
// is also appended to the error text, as described in [Error.Error].
//
// A nil cause is allowed and produces an error equivalent to the one
// returned by [NewError]. A cause that is a non-nil error interface holding
// a nil value (a nil pointer, map, slice, func or channel, such as a nil
// *Error returned by a function) is treated as a nil cause as well. This
// avoids a panic, or a dangling ": " suffix in the error text, when the
// cause's Error method is called on its nil value, and makes
// [Error.Unwrap] return a true nil.
//
// The context maps are handled exactly as in [NewError]. The context of a
// cause that is itself an *Error is not copied into the new error; each
// error in a chain keeps its own context, available from its own
// [Error.Context].
//
//	row, queryErr := db.Query(...)
//	if queryErr != nil {
//		return flam.NewWrappedError(queryErr, "unable to load user",
//			map[string]any{"id": id})
//	}
func NewWrappedError(
	cause error,
	message string,
	ctx ...map[string]any,
) *Error {
	if value.IsNil(cause) {
		cause = nil
	}

	return &Error{
		message: message,
		cause:   cause,
		context: copyContext(ctx...),
	}
}

// Error returns the error text, composed of the message and the text of the
// cause:
//
//	message and cause          "message: cause"
//	empty message and cause    "cause"
//	message and no cause       "message"
//	empty message, no cause    ""
//
// A cause whose own text is empty still produces the separator, as in
// "message: ". Because the cause's text is included, a chain of wrapped
// errors renders as "outer: middle: inner".
//
// The context is never part of the text; use [Error.Context] to read it.
//
// Error returns an empty string on a nil receiver.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	if e.cause == nil {
		return e.message
	}

	if e.message == "" {
		return e.cause.Error()
	}

	return e.message + ": " + e.cause.Error()
}

// Unwrap returns the cause of the error, or nil when the error has no cause.
//
// Unwrap goes back a single level of the chain: on an error wrapping
// another *Error, it returns that *Error, not its cause. Use [errors.Is] and
// [errors.As] to search the whole chain.
//
// Unwrap returns nil on a nil receiver.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Context returns the context fields of the error.
//
// The returned map is always non-nil and writable, including on a nil
// receiver or a zero value, where it is empty. It is a new copy on every
// call, deep for the container types listed in [NewError], so the
// caller may freely modify it, at any depth, without affecting the error or
// any other error derived from it. Values of other types, such as pointers,
// are not copied and are shared between the error and every map returned
// by Context.
//
// Since a copy is made on every call, store the result in a variable rather
// than calling Context repeatedly when reading several fields.
//
// Context returns only the fields of this error, not those of the errors
// in its chain. To collect the context of a whole chain, walk it with
// [errors.As] or [errors.Unwrap].
func (e *Error) Context() map[string]any {
	if e == nil {
		return copyContext()
	}
	return copyContext(e.context)
}

// With returns a new [Error] with the same message and cause as the
// receiver and a copy of its context in which field is set to value. When
// the field is already present, its value is replaced in the new error.
//
// The receiver is never modified, so a base error can be shared and
// specialised independently by several callers. Calls can be chained:
//
//	e := flam.NewWrappedError(cause, "unable to set value").
//		With("path", path).
//		With("segment", segment)
//
// The value is copied following the rules described in [NewError], so
// changing it after the call does not affect the returned error. The field
// name may be any string, including the empty string, and the value may be
// nil.
//
// The returned error shares the receiver's cause rather than wrapping the
// receiver, so [errors.Is] matches the cause but not the receiver itself.
//
// With returns nil on a nil receiver. Note that, as with any typed pointer,
// a nil *Error stored in a variable or return value of type error is not
// equal to nil. Keep the result as an *Error and check it before returning
// it as an error:
//
//	func f() error {
//		var e *flam.Error
//		return e.With("k", "v") // returns a non-nil error holding a nil *Error
//	}
func (e *Error) With(
	field string,
	value any,
) *Error {
	if e == nil {
		return nil
	}

	// The stored context is a private copy, never handed out (Context
	// returns copies) nor modified, so its values can be shared with the
	// new error; only the new value needs copying.
	context := make(map[string]any, len(e.context)+1)
	maps.Copy(context, e.context)
	context[field] = copyValue(value)

	return &Error{
		message: e.message,
		cause:   e.cause,
		context: context,
	}
}

// copyContext merges the given maps, in order, into a new non-nil map,
// deep copying the values with copyValue so the result shares no
// containers with its inputs.
func copyContext(
	ctx ...map[string]any,
) map[string]any {
	result := map[string]any{}
	for _, c := range ctx {
		for k, v := range c {
			result[k] = copyValue(v)
		}
	}

	return result
}

// copyValue returns a deep copy of v when it is a map with string keys and
// interface values, or a slice of interface or string elements, keeping
// its dynamic type and nil-ness, and v itself otherwise. Reflection lets it
// copy the types based on map[string]any, such as container.Bag, which
// this package cannot import. v must not contain itself.
func copyValue(
	v any,
) any {
	rv := reflect.ValueOf(v)
	switch {
	case !rv.IsValid() || !copyable(rv.Type()) || rv.IsNil():
		return v
	case rv.Kind() == reflect.Map:
		result := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		for key, elem := range rv.Seq2() {
			result.SetMapIndex(key, copyElem(elem.Interface(), rv.Type().Elem()))
		}
		return result.Interface()
	default: // a slice
		result := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		for i := range rv.Len() {
			result.Index(i).Set(copyElem(rv.Index(i).Interface(), rv.Type().Elem()))
		}
		return result.Interface()
	}
}

// copyable reports whether copyValue copies the values of type t.
func copyable(
	t reflect.Type,
) bool {
	switch t.Kind() {
	case reflect.Map:
		return t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.Interface
	case reflect.Slice:
		return t.Elem().Kind() == reflect.Interface || t.Elem().Kind() == reflect.String
	}
	return false
}

// copyElem returns a copy of an element of a map or slice, as a value of
// the element type, a nil interface element staying a nil interface.
func copyElem(
	v any,
	elemType reflect.Type,
) reflect.Value {
	copied := reflect.ValueOf(copyValue(v))
	if !copied.IsValid() {
		return reflect.Zero(elemType)
	}
	return copied
}
