// Package json is the log serializer driver of the JSON lines: its
// serializers write an entry as a single line JSON object holding its
// context values, along with the [FieldTime], [FieldLevel] and
// [FieldMessage] fields, which win over the context values of the same
// keys:
//
//	{"host":"db","level":"ERROR","message":"unable to connect","time":"2026-11-02T09:43:00.000+0000"}
//
// The keys come in order, the nested bags and maps of the context become
// nested objects, and an error becomes its message, at any depth, as
// encoding/json would encode most errors as an empty object. The other values are encoded by
// encoding/json, without escaping the HTML characters. A context it cannot
// encode, such as one holding a channel or a NaN float, is left out, its
// error held by the [FieldContextError] field, so that the entry is never
// lost.
//
// The line has no trailing newline: the stream separates the entries.
package json
