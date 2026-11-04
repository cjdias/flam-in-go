// Package stringify is the log serializer driver of the text lines: its
// serializers write an entry as its timestamp, its level in upper case
// between brackets, its message, then its context as key=value pairs:
//
//	2026-11-02T09:43:00.000+0000 [ERROR] unable to connect host=db retries=3
//
// The pairs come in key order, a nested bag, or map of string keys to
// values of any type, giving one pair per entry, with its keys joined by
// dots, such as "db.host=x", and an empty or nil one none. A value is
// formatted as by fmt.Sprint, so that an error gives its message.
//
// A key or value that is empty, or holds a space, a quote, an equal sign
// or a character that is not printable, such as a newline, is quoted as by
// strconv.Quote, so that an entry is always a single line, which no value
// can split in two. The message is quoted when it holds a quote, an equal
// sign or a character that is not printable, so that it cannot be taken
// for pairs.
//
// The line has no trailing newline: the stream separates the entries.
package stringify
