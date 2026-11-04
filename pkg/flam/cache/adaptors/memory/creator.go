package memory

import (
	"math"
	"reflect"
	"strconv"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The clock is optional, so that the adaptor factory can be
// created without the clock bundle, failing to create the memory adaptors
// only.
type creatorArgs struct {
	dig.In

	Config        flam.IConfig
	Serializers   flam.ICacheSerializerFactory
	KeyGenerators flam.ICacheKeyGeneratorFactory
	Clock         flam.IClock `optional:"true"`
}

// newCreator returns the [flam.ICacheAdaptorCreator] of the memory
// adaptors, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ICacheAdaptorCreator {
	return &creator{
		config:        args.Config,
		serializers:   args.Serializers,
		keyGenerators: args.KeyGenerators,
		clock:         args.Clock,
	}
}

// creator is the [flam.ICacheAdaptorCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config        flam.IConfig
	serializers   flam.ICacheSerializerFactory
	keyGenerators flam.ICacheKeyGeneratorFactory
	clock         flam.IClock
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the adaptor with the serializer and key generator of the
// ids of the configuration, or of their defaults, and the max entries and
// sweep period of the configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ICacheAdaptor, error) {
	id := bag.String(factory.ConfigFieldId)

	serializerID, ok := c.id(bag, cache.ConfigFieldSerializer, cache.DefaultSerializerConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, cache.ConfigFieldSerializer)
	}
	keyGeneratorID, ok := c.id(bag, cache.ConfigFieldKeyGenerator, cache.DefaultKeyGeneratorConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, cache.ConfigFieldKeyGenerator)
	}
	maxEntries, ok := positive(settings.Field(c.config, bag, ConfigFieldMaxEntries, DefaultMaxEntriesConfigPath, DefaultMaxEntries))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldMaxEntries)
	}
	sweepPeriod, ok := settings.Duration(settings.Field(c.config, bag, ConfigFieldSweepPeriod, DefaultSweepPeriodConfigPath, DefaultSweepPeriod))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldSweepPeriod)
	}

	serializer, err := c.serializers.Get(serializerID)
	if err != nil {
		return nil, newErrJoined(ErrSerializer, creatorCreateMessage, id, ErrorFieldSerializer, serializerID, err)
	}
	keyGenerator, err := c.keyGenerators.Get(keyGeneratorID)
	if err != nil {
		return nil, newErrJoined(ErrKeyGenerator, creatorCreateMessage, id, ErrorFieldKeyGenerator, keyGeneratorID, err)
	}
	if c.clock == nil {
		return nil, newErrNil(ErrClock, creatorCreateMessage, id)
	}
	return NewAdaptor(id, keyGenerator, serializer, c.clock, maxEntries, sweepPeriod)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create memory cache adaptor"

// id returns the id held by the field of the adaptor configuration, or
// else by the configuration at the default path, and false when it is not
// a non-empty string.
func (c *creator) id(
	bag flam.IBag,
	field string,
	defaultPath string,
) (string, bool) {
	val := bag.Get(field)
	if val == nil {
		val = c.config.Get(defaultPath)
	}
	s, ok := val.(string)
	return s, ok && s != ""
}

// positive returns the positive whole number held by the value, of any
// integer or float kind, or a string holding one, or false for any other
// value.
func positive(
	val any,
) (int, bool) {
	if s, ok := val.(string); ok {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		return n, err == nil && n > 0
	}

	var n float64
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n = float64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		n = rv.Float()
	default:
		return 0, false
	}
	if n != math.Trunc(n) || n < 1 || n > math.MaxInt32 {
		return 0, false
	}
	return int(n), true
}
