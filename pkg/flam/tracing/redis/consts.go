package redis

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.tracing.redis"

const (
	// ScopeName is the name of the instrumentation scope of the spans of
	// the hook, the name of the tracer it starts them with.
	ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/tracing/redis"

	// SpanNamePrefix is the prefix of the names of the spans of the hook,
	// followed by the name of the command, such as "redis.get", or by
	// [OperationPipeline].
	SpanNamePrefix = "redis."

	// OperationPipeline is the operation of the spans of the pipelines and
	// the transactions, held by their "db.operation.name" attribute.
	OperationPipeline = "pipeline"
)
