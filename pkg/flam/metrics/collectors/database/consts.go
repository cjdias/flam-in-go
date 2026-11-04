package database

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.metrics.collectors.database"

// ConnectionsConfigPath is the configuration path of the list of the ids
// of the database connections whose statistics are collected, a []string
// or a []any of strings, every connection configured in the
// [flam.IDatabaseConnectionFactory] unless set.
const ConnectionsConfigPath = "flam.metrics.collectors.database.connections"

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldID holds the id of a database connection.
	ErrorFieldID = "id"

	// ErrorFieldPath holds the configuration path of an invalid value.
	ErrorFieldPath = "path"
)
