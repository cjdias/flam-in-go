package consul

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/consul/api"
	"go.uber.org/dig"

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

// newCreator returns the [flam.IConfigSourceCreator] of the consul
// sources, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
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
// their defaults, with its own Consul client, once its values are read.
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
	strs := map[string]string{}
	for _, f := range []struct{ field, path, builtIn string }{
		{ConfigFieldAddress, DefaultAddressConfigPath, defaultAddress},
		{ConfigFieldToken, DefaultTokenConfigPath, defaultToken},
		{ConfigFieldDatacenter, DefaultDatacenterConfigPath, defaultDatacenter},
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
	address, ok := creatorAddress(strs[ConfigFieldAddress])
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldAddress)
	}
	key, ok := bag.Get(ConfigFieldKey).(string)
	if !ok || key == "" || strings.HasPrefix(key, "/") {
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

	// a transport of its own, without TLS configuration, so that the client
	// configures it, and Close closes its idle connections only
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = nil // set by the default transport once used
	client, err := api.NewClient(&api.Config{
		Address:    address.Host,
		Scheme:     address.Scheme,
		PathPrefix: strings.TrimRight(address.Path, "/"),
		Datacenter: strs[ConfigFieldDatacenter],
		Transport:  transport,
		Token:      strs[ConfigFieldToken],
		TLSConfig: api.TLSConfig{
			CAFile:             strs[ConfigFieldCAFile],
			CertFile:           certFile,
			KeyFile:            keyFile,
			InsecureSkipVerify: skipVerify, // asked for by the configuration, for tests only
		},
	})
	if err != nil {
		return nil, newErr(ErrClient, creatorCreateMessage, id, err)
	}

	if recursive && !strings.HasSuffix(key, "/") {
		key += "/"
	}
	s := &source{
		Source:    base.New(id, priority),
		kv:        client.KV(),
		transport: transport,
		key:       key,
		recursive: recursive,
		timeout:   timeout,
		parserID:  parserID,
		parser:    parser,
	}
	values, index, err := s.load(creatorCreateMessage, 0)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	s.Swap(values)
	s.index = index
	if !reload {
		return static.New(s), nil
	}
	return s, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create consul config source"

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

// creatorAddress returns the parsed address, and whether it is an absolute
// http or https URL with a host, and neither query nor fragment.
func creatorAddress(
	address string,
) (*url.URL, bool) {
	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, false
	}
	return parsed, true
}
