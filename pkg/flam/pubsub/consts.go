package pubsub

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.pubsub"

// TopicSeparator is the string separating the segments of a [PubSub]
// topic or pattern, as in "orders.created".
const TopicSeparator = "."

// Context fields of the [*flam.Error] values returned by the [PubSub]
// methods and passed to the error callbacks, as documented by each method.
const (
	// ErrorFieldID holds the subscription id given to the method, or of
	// the subscription whose handler failed.
	ErrorFieldID = "id"

	// ErrorFieldPattern holds the pattern given to Subscribe.
	ErrorFieldPattern = "pattern"

	// ErrorFieldTopic holds the topic given to Publish, or of the message
	// whose handler failed.
	ErrorFieldTopic = "topic"

	// ErrorFieldPanic holds the value a handler panicked with.
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack trace of the handler panic, as a
	// string, with [ErrHandlerPanic].
	ErrorFieldStack = "stack"
)
