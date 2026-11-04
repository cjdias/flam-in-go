package rest

import (
	"net/http"
	"net/url"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/static"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The parser factory is optional, so that registering the
// driver does not require the configuration bundle: its absence only fails
// the creation of a source.
type creatorArgs struct {
	dig.In

	Config        flam.IConfig
	ClientCreator flam.IConfigRestClientCreator
	Parsers       flam.IConfigParserFactory `optional:"true"`
}

// newCreator returns the [flam.IConfigSourceCreator] of the rest sources,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IConfigSourceCreator {
	return &creator{
		config:        args.Config,
		clientCreator: args.ClientCreator,
		parsers:       args.Parsers,
	}
}

// creator is the [flam.IConfigSourceCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config        flam.IConfig
	clientCreator flam.IConfigRestClientCreator
	parsers       flam.IConfigParserFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

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
	uri, ok := bag.Get(ConfigFieldURI).(string)
	if !ok || !creatorValidURI(uri) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldURI)
	}
	method, ok := bag.Get(ConfigFieldMethod, defaultMethod).(string)
	if !ok || !creatorValidMethod(method, uri) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldMethod)
	}
	headers, ok := creatorHeaders(bag.Get(ConfigFieldHeaders))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldHeaders)
	}
	timeout, ok := creatorDuration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimeout)
	}
	maxSize, ok := c.field(bag, ConfigFieldMaxSize, DefaultMaxSizeConfigPath, defaultMaxSize).(int)
	if !ok || maxSize <= 0 {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldMaxSize)
	}
	path, ok := bag.Get(ConfigFieldPath, defaultPath).(string)
	if !ok || !container.ValidPath(path) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPath)
	}
	timestampPath, ok := bag.Get(ConfigFieldTimestampPath, defaultTimestampPath).(string)
	if !ok || !container.ValidPath(timestampPath) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimestampPath)
	}
	parserID, ok := c.field(bag, ConfigFieldParser, DefaultParserConfigPath, defaultParser).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldParser)
	}

	if c.parsers == nil {
		return nil, newErrParser(creatorCreateMessage, id, parserID, ErrMissingFactory)
	}
	var parser flam.IConfigParser
	if parserID != "" {
		var err error
		if parser, err = c.parsers.Get(parserID); err != nil {
			return nil, newErrParser(creatorCreateMessage, id, parserID, err)
		}
	}

	client, err := c.clientCreator.Create()
	if err == nil && value.IsNil(client) {
		err = ErrNilClient
	}
	if err != nil {
		return nil, newErrClient(creatorCreateMessage, id, err)
	}

	s := &source{
		Source:        base.New(id, priority),
		client:        client,
		method:        method,
		uri:           uri,
		headers:       headers,
		timeout:       timeout,
		maxSize:       maxSize,
		path:          path,
		timestampPath: timestampPath,
		parserID:      parserID,
		parser:        parser,
		parsers:       c.parsers,
	}
	values, timestamp, err := s.load(creatorCreateMessage, nil)
	if err != nil {
		return nil, err
	}
	s.Swap(values)
	s.timestamp = timestamp
	if !reload {
		return static.New(s), nil
	}
	return s, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create rest config source"

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

// creatorDuration returns the duration of a time.Duration, or of a string
// parsed with [time.ParseDuration], as read from a configuration file, when
// it is zero or more.
func creatorDuration(
	value any,
) (time.Duration, bool) {
	duration, ok := value.(time.Duration)
	if text, isString := value.(string); isString {
		var err error
		duration, err = time.ParseDuration(text)
		ok = err == nil
	}
	return duration, ok && duration >= 0
}

// creatorValidURI reports whether the URI is absolute, with a host.
func creatorValidURI(
	uri string,
) bool {
	parsed, err := url.Parse(uri)
	return err == nil && parsed.IsAbs() && parsed.Host != ""
}

// creatorValidMethod reports whether a request with the method can be
// built.
func creatorValidMethod(
	method string,
	uri string,
) bool {
	_, err := http.NewRequest(method, uri, nil)
	return method != "" && err == nil
}

// creatorHeaders returns the headers of a bag of strings, a container.Bag
// or a map[string]any, or of nil, meaning no header.
func creatorHeaders(
	headers any,
) (http.Header, bool) {
	var bag container.Bag
	switch v := headers.(type) {
	case nil:
	case container.Bag:
		bag = v
	case map[string]any:
		bag = v
	default:
		return nil, false
	}

	result := http.Header{}
	for name, header := range bag.All() {
		str, ok := header.(string)
		if !ok || name == "" {
			return nil, false
		}
		result.Set(name, str)
	}
	return result, true
}
