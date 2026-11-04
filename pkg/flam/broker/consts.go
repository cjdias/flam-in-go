package broker

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.broker"

// Configuration path and container group of the brokers.
const (
	// BrokerConfigPath is the configuration path holding one broker
	// configuration per broker id, as read by the broker factory.
	BrokerConfigPath = "flam.broker.brokers"

	// BrokerCreatorGroup is the container group the broker driver bundles
	// provide their [flam.IBrokerCreator] in, collected by the broker
	// factory.
	BrokerCreatorGroup = "flam.broker.brokers.creator"
)

// Context fields of the [*flam.Error] values returned by the broker
// package and the broker drivers.
const (
	// ErrorFieldID holds the id of a broker, or of a process.
	ErrorFieldID = "id"

	// ErrorFieldTopic holds the topic of a publication or of a
	// subscription.
	ErrorFieldTopic = "topic"

	// ErrorFieldMessage holds the id of a message, as given by the
	// backend.
	ErrorFieldMessage = "message"

	// ErrorFieldPanic holds the value a handler panicked with.
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack trace of the goroutine of a handler
	// when it panicked, as a string.
	ErrorFieldStack = "stack"
)
