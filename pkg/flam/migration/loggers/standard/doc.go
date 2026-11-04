// Package standard is the migration logger adaptor driver writing the
// progress of the migrations to the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the container: each
// report is an entry signaled on the channel of the adaptor, at the level
// of its kind, a start, an error or a done, with the migration id, its
// description and, for an error, the error in its context, at
// [ContextFieldMigration], [ContextFieldDescription] and
// [ContextFieldError].
package standard
