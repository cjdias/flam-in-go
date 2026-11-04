package field

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.ICacheKeyGeneratorCreator] of the field
// key generators. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator() flam.ICacheKeyGeneratorCreator {
	return &creator{}
}

// creator is the [flam.ICacheKeyGeneratorCreator] of the configurations
// with the [Driver] driver.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the key generator of the fields of the configuration,
// failing at the first invalid one.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ICacheKeyGenerator, error) {
	id := bag.String(factory.ConfigFieldId)

	field, ok := bag.Get(ConfigFieldField, defaultField).(string)
	if !ok || field == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldField)
	}
	prefix, ok := bag.Get(ConfigFieldPrefix, defaultPrefix).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPrefix)
	}
	typed, ok := bag.Get(ConfigFieldType, defaultType).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldType)
	}
	return &generator{id: id, field: field, prefix: prefix, typed: typed}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create field cache key generator"
