package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/dig"
	"go.uber.org/zap"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/static"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The parser factory is optional, so that registering the
// driver does not require the configuration bundle: its absence only fails
// the creation of a source needing a parser.
type creatorArgs struct {
	dig.In

	Config  flam.IConfig
	Parsers flam.IConfigParserFactory `optional:"true"`
}

// newCreator returns the [flam.IConfigSourceCreator] of the etcd sources,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IConfigSourceCreator {
	return &creator{config: args.Config, parsers: args.Parsers}
}

// creator is the [flam.IConfigSourceCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config  flam.IConfig
	parsers flam.IConfigParserFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the source of the fields of the configuration, or of
// their defaults, with its own etcd client, once its values are read.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IConfigSource, error) {
	id := bag.String(factory.ConfigFieldId)

	priority, ok := c.field(bag, ConfigFieldPriority, DefaultPriorityConfigPath, defaultPriority).(int)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPriority)
	}
	reload, ok := c.field(bag, ConfigFieldReload, DefaultReloadConfigPath, defaultReload).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldReload)
	}
	endpoints, ok := settings.Strings(c.field(bag, ConfigFieldEndpoints, DefaultEndpointsConfigPath, []string{defaultEndpoint}))
	if !ok || !creatorValidEndpoints(endpoints) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldEndpoints)
	}
	strs := map[string]string{}
	for _, f := range []struct{ field, path, builtIn string }{
		{ConfigFieldUsername, DefaultUsernameConfigPath, defaultUsername},
		{ConfigFieldPassword, DefaultPasswordConfigPath, defaultPassword},
		{ConfigFieldParser, DefaultParserConfigPath, defaultParser},
		{ConfigFieldCAFile, DefaultCAFileConfigPath, defaultCAFile},
		{ConfigFieldCertFile, DefaultCertFileConfigPath, defaultCertFile},
		{ConfigFieldKeyFile, DefaultKeyFileConfigPath, defaultKeyFile},
	} {
		str, ok := c.field(bag, f.field, f.path, f.builtIn).(string)
		if !ok {
			return nil, newErrInvalidField(creatorCreateMessage, id, f.field)
		}
		strs[f.field] = str
	}
	key, ok := bag.Get(ConfigFieldKey).(string)
	if !ok || key == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldKey)
	}
	recursive, ok := bag.Get(ConfigFieldRecursive, defaultRecursive).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldRecursive)
	}
	parserID := strs[ConfigFieldParser]
	if parserID == "" && !recursive {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldParser)
	}
	certFile, keyFile := strs[ConfigFieldCertFile], strs[ConfigFieldKeyFile]
	if certFile == "" && keyFile != "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldCertFile)
	}
	if keyFile == "" && certFile != "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldKeyFile)
	}
	timeout, ok := settings.Duration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimeout)
	}
	skipVerify, ok := c.field(bag, ConfigFieldSkipVerify, DefaultSkipVerifyConfigPath, defaultSkipVerify).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldSkipVerify)
	}

	var parser flam.IConfigParser
	if parserID != "" {
		if c.parsers == nil {
			return nil, newErrParser(creatorCreateMessage, id, parserID, ErrMissingFactory)
		}
		var err error
		if parser, err = c.parsers.Get(parserID); err != nil {
			return nil, newErrParser(creatorCreateMessage, id, parserID, err)
		}
	}

	var tlsConfig *tls.Config
	if strs[ConfigFieldCAFile] != "" || certFile != "" || skipVerify || creatorSecure(endpoints) {
		var err error
		if tlsConfig, err = creatorTLS(strs[ConfigFieldCAFile], certFile, keyFile, skipVerify); err != nil {
			return nil, newErr(ErrTLS, creatorCreateMessage, id, err)
		}
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: timeout, // bounds the authentication
		TLS:         tlsConfig,
		Username:    strs[ConfigFieldUsername],
		Password:    strs[ConfigFieldPassword],
		Logger:      zap.NewNop(),
	})
	if err != nil {
		return nil, newErr(ErrClient, creatorCreateMessage, id, err)
	}

	if recursive && !strings.HasSuffix(key, "/") {
		key += "/"
	}
	s := &source{
		Source:    base.New(id, priority),
		client:    client,
		key:       key,
		recursive: recursive,
		timeout:   timeout,
		parserID:  parserID,
		parser:    parser,
	}
	values, stamp, err := s.load(creatorCreateMessage, nil)
	if err != nil {
		_ = client.Close() // the error of the load is the one to return
		return nil, err
	}
	s.Swap(values)
	s.stamp = stamp
	if !reload {
		return static.New(s), nil
	}
	return s, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create etcd config source"

// field returns the value of the field of the source configuration, or
// else its default.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return base.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorValidEndpoints reports whether the list of endpoints is not
// empty, and holds no empty endpoint.
func creatorValidEndpoints(
	endpoints []string,
) bool {
	if len(endpoints) == 0 {
		return false
	}
	for _, endpoint := range endpoints {
		if endpoint == "" {
			return false
		}
	}
	return true
}

// creatorSecure reports whether an endpoint is an https URL.
func creatorSecure(
	endpoints []string,
) bool {
	for _, endpoint := range endpoints {
		if strings.HasPrefix(endpoint, "https://") {
			return true
		}
	}
	return false
}

// creatorTLS returns the TLS configuration trusting the certificate
// authorities of the PEM file, when not empty, instead of those of the
// system, presenting the client certificate of the files, when not empty,
// and verifying the certificates of the cluster unless skipVerify. It
// fails with the error of the loading of a file, or errNoCertificate for a
// CA file holding no certificate.
func creatorTLS(
	caFile string,
	certFile string,
	keyFile string,
	skipVerify bool,
) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: skipVerify, // asked for by the configuration, for tests only
	}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		config.RootCAs = x509.NewCertPool()
		if !config.RootCAs.AppendCertsFromPEM(pem) {
			return nil, errNoCertificate
		}
	}
	if certFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}
