package secrets

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// storeFactoryArgs are the dependencies of the store factory, taken from
// the container. The group tag repeats StoreCreatorGroup, as a struct tag
// cannot use a constant.
type storeFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ISecretStoreCreator `group:"flam.secrets.stores.creator"`
}

// newStoreFactory returns the store factory creating the stores configured
// at StoreConfigPath with the creators of the StoreCreatorGroup group.
func newStoreFactory(
	args storeFactoryArgs,
) (flam.ISecretStoreFactory, error) {
	return factory.New(args.Config.Bag(StoreConfigPath), args.Creators...)
}
