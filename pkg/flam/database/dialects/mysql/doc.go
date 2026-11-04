// Package mysql is the database dialect driver of MySQL and MariaDB, with
// the [gorm.io/driver/mysql] driver. Its dialects connect, with their
// protocol, to their host and port, or to the socket at their host for the
// "unix" protocol, as their username, to their schema, with their
// parameters, written to the connection string escaped.
//
// The dialects parse the DATE and DATETIME values into time.Time, as
// needed by the timestamps of
// [github.com/cjdias/flam-in-go/pkg/flam.Record], unless the parameter
// "parseTime" holds "false".
package mysql
