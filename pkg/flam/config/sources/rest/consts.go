package rest

import (
	"net/http"
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.sources.rest"

// Driver is the driver name of the rest sources, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.config.sources.driver.rest"

// Fields of a rest source configuration.
const (
	// ConfigFieldPriority is the field holding the int priority of the
	// source.
	ConfigFieldPriority = "priority"

	// ConfigFieldReload is the field holding the bool telling whether the
	// source is reloaded: when false, the source does not implement
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
	// its values never change.
	ConfigFieldReload = "reload"

	// ConfigFieldURI is the required field holding the absolute URI of the
	// endpoint.
	ConfigFieldURI = "uri"

	// ConfigFieldMethod is the field holding the HTTP method of the
	// request, GET when missing.
	ConfigFieldMethod = "method"

	// ConfigFieldHeaders is the field holding the bag of the string header
	// values of the request.
	ConfigFieldHeaders = "headers"

	// ConfigFieldPath is the field holding the path, in the decoded
	// response, of the bag holding the configuration: the whole response
	// when missing or empty.
	ConfigFieldPath = "path"

	// ConfigFieldTimestampPath is the field holding the path, in the
	// decoded response, of the timestamp of the configuration, a string or
	// a number: when not empty, a Reload whose response holds the timestamp
	// of the configuration last loaded keeps it, without taking the bag at
	// [ConfigFieldPath]. Empty when missing, comparing no timestamp.
	ConfigFieldTimestampPath = "timestamp_path"

	// ConfigFieldTimeout is the field holding the time.Duration, or the
	// string such as "10s", bounding each request, from its sending to the
	// end of the reading of the response: zero leaves the client as the
	// only bound.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldMaxSize is the field holding the positive int maximum
	// size, in bytes, of a response body.
	ConfigFieldMaxSize = "max_size"

	// ConfigFieldParser is the field holding the id of the parser of the
	// responses. When empty, the parser is named after the media type of
	// each response.
	ConfigFieldParser = "parser"
)

// Configuration paths of the values taken by the fields missing from a
// source configuration.
const (
	// DefaultPriorityConfigPath is the path of the default of
	// [ConfigFieldPriority], 0 unless changed.
	DefaultPriorityConfigPath = "flam.config.defaults.sources.rest.priority"

	// DefaultReloadConfigPath is the path of the default of
	// [ConfigFieldReload], true unless changed.
	DefaultReloadConfigPath = "flam.config.defaults.sources.rest.reload"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], 30 seconds unless changed.
	DefaultTimeoutConfigPath = "flam.config.defaults.sources.rest.timeout"

	// DefaultMaxSizeConfigPath is the path of the default of
	// [ConfigFieldMaxSize], 10 MiB unless changed.
	DefaultMaxSizeConfigPath = "flam.config.defaults.sources.rest.max_size"

	// DefaultParserConfigPath is the path of the default of
	// [ConfigFieldParser], empty, so named after the media types, unless
	// changed.
	DefaultParserConfigPath = "flam.config.defaults.sources.rest.parser"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the sources.
const (
	// ErrorFieldID holds the id of the source.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldMethod holds the HTTP method of the request.
	ErrorFieldMethod = "method"

	// ErrorFieldURI holds the URI of the request.
	ErrorFieldURI = "uri"

	// ErrorFieldStatus holds the HTTP status code of the response.
	ErrorFieldStatus = "status"

	// ErrorFieldContentType holds the Content-Type of the response.
	ErrorFieldContentType = "content_type"

	// ErrorFieldParser holds the id of the parser.
	ErrorFieldParser = "parser"

	// ErrorFieldPath holds the path of the configuration in the response.
	ErrorFieldPath = "path"

	// ErrorFieldMaxSize holds the maximum size of a response body.
	ErrorFieldMaxSize = "max_size"

	// ErrorFieldTimestampPath holds the path of the timestamp in the
	// response.
	ErrorFieldTimestampPath = "timestamp_path"
)

// Defaults of the fields of a source configuration, and of the client.
const (
	defaultPriority      = 0
	defaultReload        = true
	defaultParser        = ""
	defaultMethod        = http.MethodGet
	defaultPath          = ""
	defaultTimestampPath = ""
	defaultTimeout       = 30 * time.Second
	defaultMaxSize       = 10 << 20
	defaultClientTimeout = 30 * time.Second
)
