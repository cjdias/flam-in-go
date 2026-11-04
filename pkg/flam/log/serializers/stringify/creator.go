package stringify

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.ILogSerializerCreator] of the stringify
// serializers. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator() flam.ILogSerializerCreator {
	return &creator{}
}

// creator is the [flam.ILogSerializerCreator] of the configurations with
// the [Driver] driver.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

func (c *creator) Create(
	bag flam.IBag,
) (flam.ILogSerializer, error) {
	format := bag.String(ConfigFieldFormat)
	if format == "" {
		format = DefaultFormat
	}
	return newSerializer(bag.String(factory.ConfigFieldId), format), nil
}
