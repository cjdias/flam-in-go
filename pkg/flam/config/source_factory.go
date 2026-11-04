package config

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// sourceFactoryArgs are the dependencies of the source factory, taken from
// the container. The group tag repeats SourceCreatorGroup, as a struct tag
// cannot use a constant.
type sourceFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IConfigSourceCreator `group:"flam.config.sources.creator"`
}

// newSourceFactory returns the source factory creating the sources
// configured at SourceConfigPath with the creators of the
// SourceCreatorGroup group.
func newSourceFactory(
	args sourceFactoryArgs,
) (flam.IConfigSourceFactory, error) {
	return factory.New(args.Config.Bag(SourceConfigPath), args.Creators...)
}
