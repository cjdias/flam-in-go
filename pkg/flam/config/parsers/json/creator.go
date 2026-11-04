package json

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IConfigParserCreator] of the JSON parsers.
// It is the provider registered by the bundle returned from [NewBundle].
func newCreator() flam.IConfigParserCreator {
	return &creator{}
}

// creator is the [flam.IConfigParserCreator] of the configurations with
// the [Driver] driver.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

func (c *creator) Create(
	bag flam.IBag,
) (flam.IConfigParser, error) {
	return newParser(bag.String(factory.ConfigFieldId)), nil
}
