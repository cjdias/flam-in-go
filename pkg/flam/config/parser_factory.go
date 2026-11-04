package config

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// parserFactoryArgs are the dependencies of the parser factory, taken from
// the container. The group tag repeats ParserCreatorGroup, as a struct tag
// cannot use a constant.
type parserFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IConfigParserCreator `group:"flam.config.parsers.creator"`
}

// newParserFactory returns the parser factory creating the parsers
// configured at ParserConfigPath with the creators of the
// ParserCreatorGroup group.
func newParserFactory(
	args parserFactoryArgs,
) (flam.IConfigParserFactory, error) {
	return factory.New(args.Config.Bag(ParserConfigPath), args.Creators...)
}
