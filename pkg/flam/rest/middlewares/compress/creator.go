package compress

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"strings"
	"sync"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the compress
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config: args.Config,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the middleware of the level, the minimum size, the
// content types and the encodings of the configuration, or of their
// defaults, holding a pool of compressors per encoding.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	level, ok := c.field(bag, ConfigFieldLevel, DefaultLevelConfigPath, defaultLevel).(int)
	if !ok || level < flate.HuffmanOnly || level > flate.BestCompression {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldLevel)
	}
	minSize, ok := c.field(bag, ConfigFieldMinSize, DefaultMinSizeConfigPath, defaultMinSize).(int)
	if !ok || minSize < 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMinSize)
	}
	types, ok := creatorList(c.field(bag, ConfigFieldContentTypes, DefaultContentTypesConfigPath, defaultContentTypes()))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldContentTypes)
	}
	encodings, ok := creatorList(c.field(bag, ConfigFieldEncodings, DefaultEncodingsConfigPath, defaultEncodings()))
	if !ok || len(encodings) == 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldEncodings)
	}

	m := &middleware{
		id:        id,
		minSize:   max(minSize, 1),
		types:     map[string]struct{}{},
		encodings: encodings,
		pools:     map[string]*sync.Pool{},
	}
	for _, encoding := range encodings {
		if encoding != EncodingGzip && encoding != EncodingDeflate {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldEncodings)
		}
		m.pools[encoding] = creatorPool(encoding, level)
	}
	for _, mediaType := range types {
		switch {
		case mediaType == "*/*":
			m.anyType = true
		case strings.HasSuffix(mediaType, "/*"):
			m.prefixes = append(m.prefixes, strings.TrimSuffix(mediaType, "*"))
		default:
			m.types[mediaType] = struct{}{}
		}
	}
	return m, nil
}

// field returns the value of the field of the middleware configuration,
// or else the one stored in the configuration at the default path, or
// else the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorList returns the strings of the value, a []string or a []any of
// strings, trimmed and lower cased, and whether the value is such a list
// of non-empty strings.
func creatorList(
	value any,
) ([]string, bool) {
	list, ok := settings.Strings(value)
	if !ok {
		return nil, false
	}
	normalized := make([]string, len(list))
	for i, item := range list {
		if normalized[i] = strings.ToLower(strings.TrimSpace(item)); normalized[i] == "" {
			return nil, false
		}
	}
	return normalized, true
}

// creatorPool returns the pool of the compressors of the encoding and the
// level, a valid one, so that creating a compressor never fails.
func creatorPool(
	encoding string,
	level int,
) *sync.Pool {
	return &sync.Pool{New: func() any {
		if encoding == EncodingGzip {
			zw, _ := gzip.NewWriterLevel(io.Discard, level)
			return zw
		}
		zw, _ := flate.NewWriter(io.Discard, level)
		return zw
	}}
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create compress rest middleware"
