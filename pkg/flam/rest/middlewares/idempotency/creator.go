package idempotency

import (
	"net/http"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the cache adaptor factory only when provided.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Caches flam.ICacheAdaptorFactory `optional:"true"`
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the idempotency
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config: args.Config,
		caches: args.Caches,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
	caches flam.ICacheAdaptorFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the middleware of the cache adaptor, the header, the
// methods, the requirement, the ttls, the maximum body size and the
// stored headers of the configuration, or of their defaults, failing with
// ErrInvalidField for an invalid field, ErrNoFactory without cache adaptor
// factory, and ErrCacheAdaptor when the adaptor cannot be taken from it.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	cache, ok := c.field(bag, ConfigFieldCache, DefaultCacheConfigPath, defaultCache).(string)
	if !ok || cache == "" {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldCache)
	}
	header, ok := c.field(bag, ConfigFieldHeader, DefaultHeaderConfigPath, defaultHeader).(string)
	if !ok || header == "" {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldHeader)
	}
	methods, ok := creatorList(c.field(bag, ConfigFieldMethods, DefaultMethodsConfigPath, defaultMethods()))
	if !ok || len(methods) == 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMethods)
	}
	required, ok := c.field(bag, ConfigFieldRequired, DefaultRequiredConfigPath, defaultRequired).(bool)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldRequired)
	}
	ttl, ok := settings.Duration(c.field(bag, ConfigFieldTTL, DefaultTTLConfigPath, defaultTTL))
	if !ok || ttl == 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldTTL)
	}
	lockTTL, ok := settings.Duration(c.field(bag, ConfigFieldLockTTL, DefaultLockTTLConfigPath, defaultLockTTL))
	if !ok || lockTTL == 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldLockTTL)
	}
	maxBodySize, ok := c.field(bag, ConfigFieldMaxBodySize, DefaultMaxBodySizeConfigPath, defaultMaxBodySize).(int)
	if !ok || maxBodySize < 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMaxBodySize)
	}
	headers, ok := creatorList(c.field(bag, ConfigFieldHeaders, DefaultHeadersConfigPath, defaultHeaders()))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldHeaders)
	}

	if c.caches == nil {
		return nil, newErrCache(ErrNoFactory, creatorCreateMessage, id, cache, nil)
	}
	adaptor, err := c.caches.Get(cache)
	if err != nil {
		return nil, newErrCache(ErrCacheAdaptor, creatorCreateMessage, id, cache, err)
	}

	m := &middleware{
		id:          id,
		cache:       adaptor,
		header:      header,
		methods:     map[string]struct{}{},
		required:    required,
		ttl:         ttl,
		lockTTL:     lockTTL,
		maxBodySize: maxBodySize,
		inFlight:    map[string]struct{}{},
	}
	for _, method := range methods {
		m.methods[method] = struct{}{}
	}
	for _, name := range headers {
		m.headers = append(m.headers, http.CanonicalHeaderKey(name))
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
// strings, trimmed, and whether the value is such a list of non-empty
// strings.
func creatorList(
	value any,
) ([]string, bool) {
	list, ok := settings.Strings(value)
	if !ok {
		return nil, false
	}
	trimmed := make([]string, len(list))
	for i, item := range list {
		if trimmed[i] = strings.TrimSpace(item); trimmed[i] == "" {
			return nil, false
		}
	}
	return trimmed, true
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create idempotency rest middleware"
