package bodylimit

import (
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

// newCreator returns the [flam.IRestMiddlewareCreator] of the body limit
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

// Create returns the middleware of the maximum body size of the
// configuration, or of its default.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	maxBytes, ok := settings.Field(c.config, bag, ConfigFieldMaxBytes, DefaultMaxBytesConfigPath, defaultMaxBytes).(int)
	if !ok || maxBytes <= 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMaxBytes)
	}

	return &middleware{
		id:       id,
		maxBytes: int64(maxBytes),
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create body limit rest middleware"
