package database

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.tracing.database"

const (
	// PluginName is the name of the gorm plugin returned by [NewPlugin],
	// under which gorm registers it, and the prefix of the names of its
	// callbacks, such as "flam.tracing:before_query".
	PluginName = "flam.tracing"

	// ScopeName is the name of the instrumentation scope of the spans of
	// the plugin, the name of the tracer it starts them with.
	ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/tracing/database"

	// SpanNamePrefix is the prefix of the names of the spans of the
	// plugin, followed by the operation, such as "gorm.query", and, when
	// known, by a space and the table, such as "gorm.query users".
	SpanNamePrefix = "gorm."
)

// AttributeRowsAffected is the attribute of the spans of the plugin holding
// the number of rows the statement affected, or read for a query, an
// int64.
const AttributeRowsAffected = "flam.database.rows_affected"

// Operations of the statements, held by the "db.operation.name" attribute
// of their spans, one per gorm callback processor.
const (
	// OperationCreate is the operation of the statements of the Create
	// processor, inserting rows.
	OperationCreate = "create"

	// OperationQuery is the operation of the statements of the Query
	// processor, selecting rows into a destination.
	OperationQuery = "query"

	// OperationUpdate is the operation of the statements of the Update
	// processor, updating rows.
	OperationUpdate = "update"

	// OperationDelete is the operation of the statements of the Delete
	// processor, deleting rows.
	OperationDelete = "delete"

	// OperationRow is the operation of the statements of the Row
	// processor, selecting rows read by Row and Rows.
	OperationRow = "row"

	// OperationRaw is the operation of the statements of the Raw
	// processor, run by Exec.
	OperationRaw = "raw"
)

// Context fields of the [*flam.Error] values returned by the decorated
// connection creators.
const (
	// ErrorFieldID holds the id of the connection.
	ErrorFieldID = "id"
)
