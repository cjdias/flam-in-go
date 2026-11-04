package rest

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// loggerFactoryArgs are the dependencies of the logger adaptor factory,
// taken from the container. The group tag repeats LoggerCreatorGroup, as
// a struct tag cannot use a constant.
type loggerFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IRestLoggerAdaptorCreator `group:"flam.rest.loggers.creator"`
}

// newLoggerFactory returns the logger adaptor factory creating the
// adaptors configured at LoggerConfigPath with the creators of the
// LoggerCreatorGroup group.
func newLoggerFactory(
	args loggerFactoryArgs,
) (flam.IRestLoggerAdaptorFactory, error) {
	return factory.New(args.Config.Bag(LoggerConfigPath), args.Creators...)
}

// middlewareFactoryArgs are the dependencies of the middleware factory,
// taken from the container. The group tag repeats MiddlewareCreatorGroup.
type middlewareFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IRestMiddlewareCreator `group:"flam.rest.middlewares.creator"`
}

// newMiddlewareFactory returns the middleware factory creating the
// middlewares configured at MiddlewareConfigPath with the creators of the
// MiddlewareCreatorGroup group.
func newMiddlewareFactory(
	args middlewareFactoryArgs,
) (flam.IRestMiddlewareFactory, error) {
	return factory.New(args.Config.Bag(MiddlewareConfigPath), args.Creators...)
}

// serverFactoryArgs are the dependencies of the server factory, taken
// from the container. The group tag repeats ServerCreatorGroup.
type serverFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IRestServerCreator `group:"flam.rest.servers.creator"`
}

// newServerFactory returns the server factory creating the servers
// configured at ServerConfigPath with the creators of the
// ServerCreatorGroup group.
func newServerFactory(
	args serverFactoryArgs,
) (flam.IRestServerFactory, error) {
	return factory.New(args.Config.Bag(ServerConfigPath), args.Creators...)
}
