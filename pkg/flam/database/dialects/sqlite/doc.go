// Package sqlite is the database dialect driver of SQLite, with the
// [gorm.io/driver/sqlite] driver, which needs cgo. Its dialects open the
// database file at their path, with their parameters as the query of the
// connection string, such as "_foreign_keys=on".
//
// The default path, ":memory:", opens an in-memory database per
// connection of the pool, so that each one sees an empty database: give
// the path "file::memory:" and the parameter "cache=shared" to share one
// in-memory database between them.
package sqlite
