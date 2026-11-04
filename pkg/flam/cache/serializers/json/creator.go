package json

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.ICacheSerializerCreator] of the JSON
// serializers. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator() flam.ICacheSerializerCreator {
	return &creator{}
}

// creator is the [flam.ICacheSerializerCreator] of the configurations
// with the [Driver] driver.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

func (c *creator) Create(
	bag flam.IBag,
) (flam.ICacheSerializer, error) {
	return &serializer{id: bag.String(factory.ConfigFieldId)}, nil
}
