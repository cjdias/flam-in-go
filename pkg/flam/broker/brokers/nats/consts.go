package nats

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.broker.brokers.nats"

// Driver is the driver name of the NATS brokers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.broker.brokers.driver.nats"

// Fields of a NATS broker configuration.
const (
	// ConfigFieldURL is the field holding the URL of the server, or the
	// comma separated URLs of the servers of a cluster, such as
	// "nats://127.0.0.1:4222".
	ConfigFieldURL = "url"

	// ConfigFieldName is the field holding the name of the connection,
	// shown by the server, the broker id when empty.
	ConfigFieldName = "name"

	// ConfigFieldToken is the field holding the token authenticating the
	// connection, empty for none.
	ConfigFieldToken = "token"

	// ConfigFieldUser is the field holding the user authenticating the
	// connection, along with [ConfigFieldPassword], empty for none.
	ConfigFieldUser = "user"

	// ConfigFieldPassword is the field holding the password of the user.
	ConfigFieldPassword = "password" //nolint:gosec // a configuration field name, not a credential

	// ConfigFieldCredentials is the field holding the path of the
	// credentials file, holding the JWT and the NKey seed of the user,
	// empty for none.
	ConfigFieldCredentials = "credentials"

	// ConfigFieldConnectTimeout is the field holding the time allowed to
	// connect to the server, and to the JetStream requests of the
	// creation, a positive duration such as "2s".
	ConfigFieldConnectTimeout = "connect_timeout"

	// ConfigFieldReconnectWait is the field holding the time waited
	// between two reconnections to the server, a duration such as "2s".
	ConfigFieldReconnectWait = "reconnect_wait"

	// ConfigFieldMaxReconnects is the field holding the maximum number of
	// reconnections to the server, a whole number, -1 for no limit.
	ConfigFieldMaxReconnects = "max_reconnects"

	// ConfigFieldQueue is the field holding the queue group of the core
	// NATS subscriptions, each message going to one subscription of the
	// group, empty for none, each message then going to every
	// subscription.
	ConfigFieldQueue = "queue"

	// ConfigFieldJetStream is the field holding whether the broker
	// publishes to and consumes from JetStream, a bool, false unless set.
	ConfigFieldJetStream = "jetstream"

	// ConfigFieldStream is the field holding the name of the JetStream
	// stream holding the topics of the broker, required with JetStream.
	ConfigFieldStream = "stream"

	// ConfigFieldSubjects is the field holding the list of the subjects of
	// the stream, created or updated with them when the broker is created,
	// empty to use the stream as it exists.
	ConfigFieldSubjects = "subjects"

	// ConfigFieldDurable is the field holding the prefix of the names of
	// the durable consumers of the JetStream subscriptions, the broker id
	// when empty, followed by the topic, its characters a consumer name
	// cannot hold, such as ".", replaced by underscores, and by a hash of
	// the exact prefix and topic, so that each topic has a consumer of its
	// own. The subscriptions to a topic of the brokers of the same prefix
	// share their consumer, each message going to one of them.
	ConfigFieldDurable = "durable"

	// ConfigFieldAckWait is the field holding the time JetStream waits for
	// the acknowledgement of a message before delivering it again, a
	// positive duration such as "30s".
	ConfigFieldAckWait = "ack_wait"

	// ConfigFieldMaxDeliver is the field holding the maximum number of
	// deliveries of a JetStream message, a whole number, -1 for no limit.
	ConfigFieldMaxDeliver = "max_deliver"
)

// Configuration paths of the values taken by the fields missing from a
// broker configuration.
const (
	// DefaultURLConfigPath is the path of the default of [ConfigFieldURL],
	// "nats://127.0.0.1:4222" unless set.
	DefaultURLConfigPath = "flam.broker.defaults.brokers.nats.url"

	// DefaultConnectTimeoutConfigPath is the path of the default of
	// [ConfigFieldConnectTimeout], "2s" unless set.
	DefaultConnectTimeoutConfigPath = "flam.broker.defaults.brokers.nats.connect_timeout"

	// DefaultReconnectWaitConfigPath is the path of the default of
	// [ConfigFieldReconnectWait], "2s" unless set.
	DefaultReconnectWaitConfigPath = "flam.broker.defaults.brokers.nats.reconnect_wait"

	// DefaultMaxReconnectsConfigPath is the path of the default of
	// [ConfigFieldMaxReconnects], 60 unless set.
	DefaultMaxReconnectsConfigPath = "flam.broker.defaults.brokers.nats.max_reconnects"

	// DefaultJetStreamConfigPath is the path of the default of
	// [ConfigFieldJetStream], false unless set.
	DefaultJetStreamConfigPath = "flam.broker.defaults.brokers.nats.jetstream"

	// DefaultAckWaitConfigPath is the path of the default of
	// [ConfigFieldAckWait], "30s" unless set.
	DefaultAckWaitConfigPath = "flam.broker.defaults.brokers.nats.ack_wait"

	// DefaultMaxDeliverConfigPath is the path of the default of
	// [ConfigFieldMaxDeliver], -1 unless set.
	DefaultMaxDeliverConfigPath = "flam.broker.defaults.brokers.nats.max_deliver"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the broker.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldURL holds the URL of the server.
	ErrorFieldURL = "url"

	// ErrorFieldStream holds the name of the JetStream stream.
	ErrorFieldStream = "stream"
)

// Defaults of the fields of a broker configuration.
const (
	defaultURL            = "nats://127.0.0.1:4222"
	defaultConnectTimeout = 2 * time.Second
	defaultReconnectWait  = 2 * time.Second
	defaultMaxReconnects  = 60
	defaultJetStream      = false
	defaultAckWait        = 30 * time.Second
	defaultMaxDeliver     = -1
)
