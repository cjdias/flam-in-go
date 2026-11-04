package compress

import (
	"compress/flate"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.compress"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "compress"

	// Driver is the driver name of the compress middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.compress"
)

// Content codings of the responses, listed by [ConfigFieldEncodings].
const (
	// EncodingGzip is the gzip content coding, of [compress/gzip].
	EncodingGzip = "gzip"

	// EncodingDeflate is the deflate content coding, the zlib-less
	// [compress/flate] stream most clients expect.
	EncodingDeflate = "deflate"
)

// Fields of a compress middleware configuration.
const (
	// ConfigFieldLevel is the field holding the int compression level,
	// from -2, Huffman only, to 9, the best compression, -1 being the
	// default of [compress/flate] and 0 storing the data uncompressed.
	ConfigFieldLevel = "level"

	// ConfigFieldMinSize is the field holding the int number of bytes of
	// body, 0 or more, below which a response is sent uncompressed.
	ConfigFieldMinSize = "min_size"

	// ConfigFieldContentTypes is the field holding the list of the media
	// types of the responses compressed, matched whatever their case and
	// parameters, such as "application/json", a "type/*" entry matching
	// any subtype of the type and "*/*" any media type, a []string or a
	// []any of non-empty strings.
	ConfigFieldContentTypes = "content_types"

	// ConfigFieldEncodings is the field holding the list of the content
	// codings offered, [EncodingGzip] or [EncodingDeflate], the first one
	// preferred when the client accepts several equally, a []string or a
	// []any of at least one of them.
	ConfigFieldEncodings = "encodings"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], -1 unless changed.
	DefaultLevelConfigPath = "flam.rest.defaults.middlewares.compress.level"

	// DefaultMinSizeConfigPath is the path of the default of
	// [ConfigFieldMinSize], 1024 unless changed.
	DefaultMinSizeConfigPath = "flam.rest.defaults.middlewares.compress.min_size"

	// DefaultContentTypesConfigPath is the path of the default of
	// [ConfigFieldContentTypes], "application/json",
	// "application/problem+json" and "text/*" unless changed.
	DefaultContentTypesConfigPath = "flam.rest.defaults.middlewares.compress.content_types"

	// DefaultEncodingsConfigPath is the path of the default of
	// [ConfigFieldEncodings], [EncodingGzip] and [EncodingDeflate] unless
	// changed.
	DefaultEncodingsConfigPath = "flam.rest.defaults.middlewares.compress.encodings"
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
	defaultLevel   = flate.DefaultCompression
	defaultMinSize = 1024
)

// defaultContentTypes returns the default of [ConfigFieldContentTypes], a
// new list each time, so that the configuration never shares it.
func defaultContentTypes() []string {
	return []string{"application/json", "application/problem+json", "text/*"}
}

// defaultEncodings returns the default of [ConfigFieldEncodings], a new
// list each time, so that the configuration never shares it.
func defaultEncodings() []string {
	return []string{EncodingGzip, EncodingDeflate}
}
