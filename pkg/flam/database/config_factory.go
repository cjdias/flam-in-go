package database

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// configFactoryArgs are the dependencies of the config factory,
// taken from the container. The group tag repeats ConfigCreatorGroup,
// as a struct tag cannot use a constant.
type configFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IDatabaseConfigCreator `group:"flam.database.configs.creator"`
}

// newConfigFactory returns the config factory creating the
// configs configured at ConfigConfigPath with the creators of the
// ConfigCreatorGroup group.
func newConfigFactory(
	args configFactoryArgs,
) (flam.IDatabaseConfigFactory, error) {
	return factory.New(args.Config.Bag(ConfigConfigPath), args.Creators...)
}
