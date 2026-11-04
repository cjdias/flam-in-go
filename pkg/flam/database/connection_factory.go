package database

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// connectionFactoryArgs are the dependencies of the connection factory,
// taken from the container. The group tag repeats ConnectionCreatorGroup,
// as a struct tag cannot use a constant.
type connectionFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IDatabaseConnectionCreator `group:"flam.database.connections.creator"`
}

// newConnectionFactory returns the connection factory creating the
// connections configured at ConnectionConfigPath with the creators of the
// ConnectionCreatorGroup group.
func newConnectionFactory(
	args connectionFactoryArgs,
) (flam.IDatabaseConnectionFactory, error) {
	return factory.New(args.Config.Bag(ConnectionConfigPath), args.Creators...)
}
