package standard

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math"
	"os"
	"reflect"

	gomongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo"
)

// newCreator returns the [flam.IMongoConnectionCreator] of the standard
// connections, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.IMongoConnectionCreator {
	return &creator{config: config}
}

// creator is the [flam.IMongoConnectionCreator] of the configurations
// with the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the connection to the deployment of the fields, failing
// at the first invalid one. It pings the deployment only when the ping
// field is true, disconnecting the client when the ping fails.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IMongoConnection, error) {
	id := bag.String(factory.ConfigFieldId)
	r := &reader{config: c.config, bag: bag}

	opts := options.Client().ApplyURI(r.text(ConfigFieldURI, DefaultURIConfigPath, defaultURI, true))
	database := r.text(ConfigFieldDatabase, DefaultDatabaseConfigPath, defaultDatabase, true)
	appName := r.text(ConfigFieldAppName, DefaultAppNameConfigPath, defaultAppName, false)
	credential := options.Credential{
		Username:      r.text(ConfigFieldUsername, DefaultUsernameConfigPath, defaultUsername, false),
		Password:      r.text(ConfigFieldPassword, DefaultPasswordConfigPath, defaultPassword, false),
		AuthSource:    r.text(ConfigFieldAuthSource, "", "", false),
		AuthMechanism: r.text(ConfigFieldAuthMechanism, "", "", false),
	}
	readerOptional(r, ConfigFieldConnectTimeout, settings.Duration, opts.SetConnectTimeout)
	readerOptional(r, ConfigFieldServerSelectionTimeout, settings.Duration, opts.SetServerSelectionTimeout)
	readerOptional(r, ConfigFieldTimeout, settings.Duration, opts.SetTimeout)
	readerOptional(r, ConfigFieldHeartbeatInterval, settings.Duration, opts.SetHeartbeatInterval)
	readerOptional(r, ConfigFieldMaxConnIdleTime, settings.Duration, opts.SetMaxConnIdleTime)
	readerOptional(r, ConfigFieldMaxPoolSize, readerCount, opts.SetMaxPoolSize)
	readerOptional(r, ConfigFieldMinPoolSize, readerCount, opts.SetMinPoolSize)
	readerOptional(r, ConfigFieldMaxConnecting, readerCount, opts.SetMaxConnecting)
	useTLS := r.bool(ConfigFieldTLS, "", false)
	tlsFiles := map[string]string{
		ConfigFieldTLSCAFile:   r.text(ConfigFieldTLSCAFile, "", "", false),
		ConfigFieldTLSCertFile: r.text(ConfigFieldTLSCertFile, "", "", false),
		ConfigFieldTLSKeyFile:  r.text(ConfigFieldTLSKeyFile, "", "", false),
	}
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         r.text(ConfigFieldTLSServerName, "", "", false),
		InsecureSkipVerify: r.bool(ConfigFieldTLSInsecure, "", false), //nolint:gosec // asked for by the configuration, for tests
	}
	ping := r.bool(ConfigFieldPing, DefaultPingConfigPath, defaultPing)
	if r.invalid != "" {
		return nil, newErrInvalidField(id, r.invalid)
	}

	if appName != "" {
		opts.SetAppName(appName)
	}
	if credential.Username != "" || credential.AuthMechanism != "" {
		credential.PasswordSet = credential.Password != ""
		opts.SetAuth(credential)
	}
	if useTLS {
		if field, err := creatorTLS(tlsConfig, tlsFiles); err != nil {
			return nil, newErrTLS(id, field, err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	client, err := gomongo.Connect(opts)
	if err != nil {
		return nil, newErrConnect(id, err)
	}
	// the client is not nil and the database not empty, so it cannot fail
	conn, _ := mongo.NewConnection(id, client, database)
	if ping {
		if err := conn.Ping(context.Background()); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

// reader reads the fields of a connection configuration, recording the
// first invalid one, after which it keeps reading, returning zero values
// for the invalid ones.
type reader struct {
	config  flam.IConfig
	bag     flam.IBag
	invalid string
}

// fail records the field as invalid, unless one already is.
func (r *reader) fail(
	field string,
) {
	if r.invalid == "" {
		r.invalid = field
	}
}

// text returns the string of the field, or of its default, which must not
// be empty when required.
func (r *reader) text(
	field string,
	defaultPath string,
	builtIn string,
	required bool,
) string {
	s, ok := settings.Field(r.config, r.bag, field, defaultPath, builtIn).(string)
	if !ok || (required && s == "") {
		r.fail(field)
	}
	return s
}

// bool returns the bool of the field, or of its default.
func (r *reader) bool(
	field string,
	defaultPath string,
	builtIn bool,
) bool {
	b, ok := settings.Field(r.config, r.bag, field, defaultPath, builtIn).(bool)
	if !ok {
		r.fail(field)
	}
	return b
}

// readerOptional sets, with set, the value of the field, as read by parse,
// unless the field is missing, recording the field as invalid when parse
// fails.
func readerOptional[T any](
	r *reader,
	field string,
	parse func(any) (T, bool),
	set func(T) *options.ClientOptions,
) {
	val := r.bag.Get(field)
	if val == nil {
		return
	}
	parsed, ok := parse(val)
	if !ok {
		r.fail(field)
		return
	}
	set(parsed)
}

// readerCount returns the whole number held by the value, of any integer
// or float kind, between 0 and math.MaxUint32, and false for any other
// value.
func readerCount(
	val any,
) (uint64, bool) {
	var n float64
	rv := reflect.ValueOf(val)
	switch {
	case rv.CanInt():
		n = float64(rv.Int())
	case rv.CanUint():
		n = float64(rv.Uint())
	case rv.CanFloat():
		n = rv.Float()
	default:
		return 0, false
	}
	if n != math.Trunc(n) || n < 0 || n > math.MaxUint32 {
		return 0, false
	}
	return uint64(n), true
}

// creatorTLS loads in config the certificates of the files, by field, the
// authorities of the CA file and the client certificate of the cert and
// key files, when given, returning the field of the file that cannot be
// read, along with its error.
func creatorTLS(
	config *tls.Config,
	files map[string]string,
) (string, error) {
	if path := files[ConfigFieldTLSCAFile]; path != "" {
		pem, err := os.ReadFile(path) //nolint:gosec // a path of the configuration
		if err != nil {
			return ConfigFieldTLSCAFile, err
		}
		config.RootCAs = x509.NewCertPool()
		if !config.RootCAs.AppendCertsFromPEM(pem) {
			return ConfigFieldTLSCAFile, errNoCertificate
		}
	}
	certFile, keyFile := files[ConfigFieldTLSCertFile], files[ConfigFieldTLSKeyFile]
	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return ConfigFieldTLSCertFile, err
		}
		config.Certificates = []tls.Certificate{cert}
	}
	return "", nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard mongo connection"
