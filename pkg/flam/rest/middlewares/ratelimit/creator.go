package ratelimit

import (
	"container/list"
	"math"
	"time"

	"go.uber.org/dig"
	"golang.org/x/time/rate"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Clock  flam.IClock
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the ratelimit
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config: args.Config,
		clock:  args.Clock,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
	clock  flam.IClock
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the middleware of the rate, the burst, the maximum
// number of buckets, the trusted proxy header and the IPv6 prefix length
// of the configuration, or of their defaults, holding no bucket yet.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	perSecond, ok := creatorRate(c.field(bag, ConfigFieldRate, DefaultRateConfigPath, defaultRate))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldRate)
	}
	burst, ok := c.field(bag, ConfigFieldBurst, DefaultBurstConfigPath, defaultBurst).(int)
	if !ok || burst < 1 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldBurst)
	}
	maxKeys, ok := c.field(bag, ConfigFieldMaxKeys, DefaultMaxKeysConfigPath, defaultMaxKeys).(int)
	if !ok || maxKeys < 1 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMaxKeys)
	}
	header, ok := c.field(bag, ConfigFieldTrustedProxyHeader, DefaultTrustedProxyHeaderConfigPath, defaultTrustedProxyHeader).(string)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldTrustedProxyHeader)
	}
	prefix, ok := c.field(bag, ConfigFieldIPv6Prefix, DefaultIPv6PrefixConfigPath, defaultIPv6Prefix).(int)
	if !ok || prefix < 0 || prefix > 128 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldIPv6Prefix)
	}

	return &middleware{
		id:      id,
		clock:   c.clock,
		limit:   rate.Limit(perSecond),
		burst:   burst,
		refill:  creatorRefill(float64(burst) / perSecond),
		maxKeys: maxKeys,
		header:  header,
		prefix:  prefix,
		buckets: map[string]*list.Element{},
		recent:  list.New(),
	}, nil
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

// creatorRate returns the rate of the value, a float64, an int or an
// int64, and whether it is one, finite and over 0.
func creatorRate(
	value any,
) (float64, bool) {
	var perSecond float64
	switch v := value.(type) {
	case float64:
		perSecond = v
	case int:
		perSecond = float64(v)
	case int64:
		perSecond = float64(v)
	}
	return perSecond, perSecond > 0 && !math.IsInf(perSecond, 1)
}

// creatorRefill returns the duration of the seconds an empty bucket takes
// to refill, the maximum duration when longer.
func creatorRefill(
	seconds float64,
) time.Duration {
	if nanoseconds := seconds * float64(time.Second); nanoseconds < float64(math.MaxInt64) {
		return time.Duration(nanoseconds)
	}
	return time.Duration(math.MaxInt64)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create ratelimit rest middleware"
