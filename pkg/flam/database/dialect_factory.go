package database

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// dialectFactoryArgs are the dependencies of the dialect factory, taken
// from the container. The group tag repeats DialectCreatorGroup, as a
// struct tag cannot use a constant.
type dialectFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IDatabaseDialectCreator `group:"flam.database.dialects.creator"`
}

// newDialectFactory returns the dialect factory creating the dialects
// configured at DialectConfigPath with the creators of the
// DialectCreatorGroup group.
func newDialectFactory(
	args dialectFactoryArgs,
) (flam.IDatabaseDialectFactory, error) {
	return factory.New(args.Config.Bag(DialectConfigPath), args.Creators...)
}
