package standard

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.mongo.connections.standard"

// Driver is the driver name of the standard connections, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.mongo.connections.driver.standard"

// Fields of a standard connection configuration. A field setting a client
// option takes precedence over the same option of the URI, and a missing
// one leaves the option of the URI, or else the driver default. A whole
// number is of any integer or float kind, and a duration is a
// [time.Duration] or a string such as "5s", neither negative.
const (
	// ConfigFieldURI is the field holding the connection string of the
	// deployment, such as "mongodb://db-1,db-2/?replicaSet=rs0", which may
	// hold the credentials and any client option.
	ConfigFieldURI = "uri"

	// ConfigFieldDatabase is the field holding the name of the default
	// database of the connection.
	ConfigFieldDatabase = "database"

	// ConfigFieldAppName is the field holding the name of the application
	// sent to the deployment, empty for none.
	ConfigFieldAppName = "app_name"

	// ConfigFieldUsername is the field holding the username to
	// authenticate with, empty to leave the credentials of the URI.
	ConfigFieldUsername = "username"

	// ConfigFieldPassword is the field holding the password to
	// authenticate with, empty for none.
	ConfigFieldPassword = "password" //nolint:gosec // a configuration field name, not a credential

	// ConfigFieldAuthSource is the field holding the database the
	// credentials are checked against, empty for the default of the
	// mechanism.
	ConfigFieldAuthSource = "auth_source"

	// ConfigFieldAuthMechanism is the field holding the authentication
	// mechanism, such as "SCRAM-SHA-256", empty to negotiate it. The
	// credentials of the fields replace those of the URI when it, or the
	// username, is not empty.
	ConfigFieldAuthMechanism = "auth_mechanism"

	// ConfigFieldConnectTimeout is the field holding the duration of the
	// timeout of opening a connection to a server.
	ConfigFieldConnectTimeout = "connect_timeout"

	// ConfigFieldServerSelectionTimeout is the field holding the duration
	// an operation, Ping included, waits for a suitable server.
	ConfigFieldServerSelectionTimeout = "server_selection_timeout"

	// ConfigFieldTimeout is the field holding the duration of the timeout
	// of an operation, 0 for none.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldHeartbeatInterval is the field holding the duration
	// between two checks of a server by the client.
	ConfigFieldHeartbeatInterval = "heartbeat_interval"

	// ConfigFieldMaxConnIdleTime is the field holding the duration after
	// which an idle connection is closed, 0 for never.
	ConfigFieldMaxConnIdleTime = "max_conn_idle_time"

	// ConfigFieldMaxPoolSize is the field holding the whole maximum number
	// of connections of the pool of a server, 0 for no limit.
	ConfigFieldMaxPoolSize = "max_pool_size"

	// ConfigFieldMinPoolSize is the field holding the whole minimum number
	// of connections of the pool of a server.
	ConfigFieldMinPoolSize = "min_pool_size"

	// ConfigFieldMaxConnecting is the field holding the whole maximum
	// number of connections of the pool of a server being opened at once.
	ConfigFieldMaxConnecting = "max_connecting"

	// ConfigFieldTLS is the field holding whether the connections use TLS,
	// a bool, with the TLS fields following it, false to leave the TLS
	// options of the URI.
	ConfigFieldTLS = "tls"

	// ConfigFieldTLSCAFile is the field holding the path of the PEM file
	// of the certificate authorities trusted, empty for those of the
	// system.
	ConfigFieldTLSCAFile = "tls_ca_file"

	// ConfigFieldTLSCertFile is the field holding the path of the PEM file
	// of the client certificate, along with ConfigFieldTLSKeyFile, empty
	// for none.
	ConfigFieldTLSCertFile = "tls_cert_file"

	// ConfigFieldTLSKeyFile is the field holding the path of the PEM file
	// of the key of the client certificate.
	ConfigFieldTLSKeyFile = "tls_key_file"

	// ConfigFieldTLSServerName is the field holding the name the
	// certificates of the servers are checked against, empty for their
	// host.
	ConfigFieldTLSServerName = "tls_server_name"

	// ConfigFieldTLSInsecure is the field holding whether the certificates
	// of the servers are not checked, a bool, for tests only.
	ConfigFieldTLSInsecure = "tls_insecure"

	// ConfigFieldPing is the field holding whether the creation of the
	// connection pings the deployment, failing when it cannot be reached
	// within the server selection timeout, a bool.
	ConfigFieldPing = "ping"
)

// Configuration paths of the values taken by the fields missing from a
// connection configuration.
const (
	// DefaultURIConfigPath is the path of the default of [ConfigFieldURI],
	// "mongodb://127.0.0.1:27017" unless changed.
	DefaultURIConfigPath = "flam.mongo.defaults.connections.standard.uri"

	// DefaultDatabaseConfigPath is the path of the default of
	// [ConfigFieldDatabase], "flam" unless changed.
	DefaultDatabaseConfigPath = "flam.mongo.defaults.connections.standard.database"

	// DefaultAppNameConfigPath is the path of the default of
	// [ConfigFieldAppName], empty unless changed.
	DefaultAppNameConfigPath = "flam.mongo.defaults.connections.standard.app_name"

	// DefaultUsernameConfigPath is the path of the default of
	// [ConfigFieldUsername], empty unless changed.
	DefaultUsernameConfigPath = "flam.mongo.defaults.connections.standard.username"

	// DefaultPasswordConfigPath is the path of the default of
	// [ConfigFieldPassword], empty unless changed.
	DefaultPasswordConfigPath = "flam.mongo.defaults.connections.standard.password" //nolint:gosec // a configuration path, not a credential

	// DefaultPingConfigPath is the path of the default of
	// [ConfigFieldPing], false unless changed.
	DefaultPingConfigPath = "flam.mongo.defaults.connections.standard.ping"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the connection.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a connection configuration.
const (
	defaultURI      = "mongodb://127.0.0.1:27017"
	defaultDatabase = "flam"
	defaultAppName  = ""
	defaultUsername = ""
	defaultPassword = ""
	defaultPing     = false
)
