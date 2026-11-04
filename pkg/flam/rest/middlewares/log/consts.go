package log

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.log"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "log"

	// Driver is the driver name of the log middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.log"
)

// Fields of a log middleware configuration.
const (
	// ConfigFieldChannel is the field holding the log channel of the
	// entries.
	ConfigFieldChannel = "channel"

	// ConfigFieldLevelRequest is the field holding the level of the
	// entries of the requests received, a whole number, a [flam.LogLevel]
	// included, or a level name such as "info", as [flam.LogLevel] From
	// reads it.
	ConfigFieldLevelRequest = "level_request"

	// ConfigFieldLevelResponse is the field holding the level of the
	// entries of the responses of a status under 500, read as
	// [ConfigFieldLevelRequest].
	ConfigFieldLevelResponse = "level_response"

	// ConfigFieldLevelError is the field holding the level of the entries
	// of the responses of a status of 500 or more, and of the handlers
	// that panicked, read as [ConfigFieldLevelRequest].
	ConfigFieldLevelError = "level_error"

	// ConfigFieldParseJSON is the field holding the bool telling whether
	// the JSON bodies logged are also decoded, at [ContextFieldBodyJSON].
	ConfigFieldParseJSON = "parse_json"

	// ConfigFieldMaxBody is the field holding the int maximum number of
	// bytes of a body logged, 0 or more, 0 leaving the bodies out of the
	// entries.
	ConfigFieldMaxBody = "max_body"

	// ConfigFieldRedactHeaders is the field holding the list of the names
	// of the headers whose values are logged as [RedactedValue], whatever
	// their case, a []string or a []any of strings.
	ConfigFieldRedactHeaders = "redact_headers"

	// ConfigFieldRedactParams is the field holding the list of the names
	// of the query parameters whose values are logged as [RedactedValue],
	// whatever their case, a []string or a []any of strings.
	ConfigFieldRedactParams = "redact_params"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultChannelConfigPath is the path of the default of
	// [ConfigFieldChannel], "flam.rest" unless changed.
	DefaultChannelConfigPath = "flam.rest.defaults.middlewares.log.channel"

	// DefaultLevelRequestConfigPath is the path of the default of
	// [ConfigFieldLevelRequest], "info" unless changed.
	DefaultLevelRequestConfigPath = "flam.rest.defaults.middlewares.log.level_request"

	// DefaultLevelResponseConfigPath is the path of the default of
	// [ConfigFieldLevelResponse], "info" unless changed.
	DefaultLevelResponseConfigPath = "flam.rest.defaults.middlewares.log.level_response"

	// DefaultLevelErrorConfigPath is the path of the default of
	// [ConfigFieldLevelError], "error" unless changed.
	DefaultLevelErrorConfigPath = "flam.rest.defaults.middlewares.log.level_error"

	// DefaultParseJSONConfigPath is the path of the default of
	// [ConfigFieldParseJSON], false unless changed.
	DefaultParseJSONConfigPath = "flam.rest.defaults.middlewares.log.parse_json"

	// DefaultMaxBodyConfigPath is the path of the default of
	// [ConfigFieldMaxBody], 65536 unless changed.
	DefaultMaxBodyConfigPath = "flam.rest.defaults.middlewares.log.max_body"

	// DefaultRedactHeadersConfigPath is the path of the default of
	// [ConfigFieldRedactHeaders], "Authorization", "Cookie", "Set-Cookie"
	// and "Proxy-Authorization" unless changed.
	DefaultRedactHeadersConfigPath = "flam.rest.defaults.middlewares.log.redact_headers"

	// DefaultRedactParamsConfigPath is the path of the default of
	// [ConfigFieldRedactParams], "access_token", "api_key", "apikey",
	// "password", "secret" and "token" unless changed.
	DefaultRedactParamsConfigPath = "flam.rest.defaults.middlewares.log.redact_params"
)

// RedactedValue is the value logged in place of the values of a header
// listed by [ConfigFieldRedactHeaders], or of a query parameter listed by
// [ConfigFieldRedactParams].
const RedactedValue = "[redacted]"

// Context fields of the log entries.
const (
	// ContextFieldRequest holds the bag describing the request, in both
	// entries, with the fields [ContextFieldMethod], [ContextFieldPath],
	// [ContextFieldPattern], [ContextFieldEndpoint],
	// [ContextFieldService], [ContextFieldRemote], [ContextFieldHeaders],
	// [ContextFieldParams], and [ContextFieldRequestId],
	// [ContextFieldBody], [ContextFieldBodyTruncated] and
	// [ContextFieldBodyJSON] as they apply. The body of a request expecting a "100-continue" is not
	// read, so that its handler may reject it before the client sends it:
	// the bag then holds none of the body fields.
	ContextFieldRequest = "request"

	// ContextFieldResponse holds the bag describing the response, in the
	// entry signaled once the request is served, with the fields
	// [ContextFieldStatus], [ContextFieldSize], [ContextFieldHeaders],
	// and, when the status is not the one of the endpoint,
	// [ContextFieldBody], [ContextFieldBodyTruncated] and
	// [ContextFieldBodyJSON] as they apply.
	ContextFieldResponse = "response"

	// ContextFieldDuration holds the time taken to serve the request, as
	// a [time.Duration], in the entry signaled once it is served.
	ContextFieldDuration = "duration"

	// ContextFieldPanicked holds true, in the entry signaled once the
	// request is served, when the handler panicked, and is left out
	// otherwise.
	ContextFieldPanicked = "panicked"

	// ContextFieldMethod holds the method of the request.
	ContextFieldMethod = "method"

	// ContextFieldPath holds the path of the URL of the request.
	ContextFieldPath = "path"

	// ContextFieldPattern holds the pattern of the endpoint.
	ContextFieldPattern = "pattern"

	// ContextFieldEndpoint holds the id of the endpoint.
	ContextFieldEndpoint = "endpoint"

	// ContextFieldService holds the id of the service of the endpoint.
	ContextFieldService = "service"

	// ContextFieldRemote holds the remote address of the request.
	ContextFieldRemote = "remote"

	// ContextFieldRequestId holds the id of the request, read by
	// [github.com/cjdias/flam-in-go/pkg/flam/rest.RequestIdOf] from its
	// context, as stored by the request id middleware listed before the
	// log middleware, and is left out when the context holds none.
	ContextFieldRequestId = "request_id"

	// ContextFieldHeaders holds the bag of the headers of the request, or
	// of the response, each name holding its value, or the list of its
	// values when it has several, or [RedactedValue] when redacted.
	ContextFieldHeaders = "headers"

	// ContextFieldParams holds the bag of the query parameters of the
	// request, each name holding its value, or the list of its values
	// when it has several, or [RedactedValue] when redacted.
	ContextFieldParams = "params"

	// ContextFieldBody holds the body, as a string of at most
	// [ConfigFieldMaxBody] bytes, left out when that maximum is 0, and
	// from the bag of a request expecting a "100-continue".
	ContextFieldBody = "body"

	// ContextFieldBodyTruncated holds true when the body is longer than
	// the one logged, and is left out otherwise.
	ContextFieldBodyTruncated = "body_truncated"

	// ContextFieldBodyJSON holds the body decoded from JSON, when
	// [ConfigFieldParseJSON] holds true, the content type is JSON and the
	// body logged, whole, decodes, and is left out otherwise.
	ContextFieldBodyJSON = "body_json"

	// ContextFieldStatus holds the status of the response, or 0 when the
	// handler panicked before writing it, as no response is then sent.
	ContextFieldStatus = "status"

	// ContextFieldSize holds the number of bytes of the body written.
	ContextFieldSize = "size"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultChannel       = "flam.rest"
	defaultLevelRequest  = flam.LogInfo
	defaultLevelResponse = flam.LogInfo
	defaultLevelError    = flam.LogError
	defaultParseJSON     = false
	defaultMaxBody       = 65536
)

// defaultRedactHeaders returns the default of [ConfigFieldRedactHeaders],
// a new list each time, so that the configuration never shares it.
func defaultRedactHeaders() []string {
	return []string{"Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization"}
}

// defaultRedactParams returns the default of [ConfigFieldRedactParams], a
// new list each time, so that the configuration never shares it.
func defaultRedactParams() []string {
	return []string{"access_token", "api_key", "apikey", "password", "secret", "token"}
}
