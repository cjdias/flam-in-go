package file

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newCreator returns the [flam.ISecretStoreCreator] of the file stores,
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

// Create returns the store of the directory and trimming of the
// configuration, or of their defaults. The directory need not exist yet.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ISecretStore, error) {
	id := bag.String(factory.ConfigFieldId)

	dir, ok := c.field(bag, ConfigFieldDir, DefaultDirConfigPath, defaultDir).(string)
	if !ok || dir == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldDir)
	}
	trim, ok := c.field(bag, ConfigFieldTrim, DefaultTrimConfigPath, defaultTrim).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTrim)
	}
	return &store{id: id, dir: dir, trim: trim}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create file secret store"

// field returns the value of the field of the store configuration, or else
// its default.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}
