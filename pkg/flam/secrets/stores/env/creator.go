package env

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newCreator returns the [flam.ISecretStoreCreator] of the env stores,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.ISecretStoreCreator {
	return &creator{config: config}
}

// creator is the [flam.ISecretStoreCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the store of the prefix of the configuration, or of its
// default.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ISecretStore, error) {
	id := bag.String(factory.ConfigFieldId)

	prefix, ok := settings.Field(c.config, bag, ConfigFieldPrefix, DefaultPrefixConfigPath, defaultPrefix).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPrefix)
	}
	return &store{id: id, prefix: prefix}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create env secret store"
