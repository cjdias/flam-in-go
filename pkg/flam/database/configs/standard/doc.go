// Package standard is the database configuration driver of the gorm
// configurations read field by field from their configuration, each field
// of gorm.Config having a snake case field, such as
// "skip_default_transaction", the missing ones keeping the gorm default.
//
// The "logger" field holds the configuration of the gorm logger: its type,
// writing to the standard output or discarding, its level, its slow query
// threshold and its other gorm options.
package standard
