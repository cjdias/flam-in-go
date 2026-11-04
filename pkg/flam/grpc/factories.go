package grpc

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
	Creators []flam.IGrpcLoggerAdaptorCreator `group:"flam.grpc.loggers.creator"`
}

// newLoggerFactory returns the logger adaptor factory creating the
// adaptors configured at LoggerConfigPath with the creators of the
// LoggerCreatorGroup group.
func newLoggerFactory(
	args loggerFactoryArgs,
) (flam.IGrpcLoggerAdaptorFactory, error) {
	return factory.New(args.Config.Bag(LoggerConfigPath), args.Creators...)
}

// interceptorFactoryArgs are the dependencies of the interceptor factory,
// taken from the container. The group tag repeats InterceptorCreatorGroup.
type interceptorFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IGrpcInterceptorCreator `group:"flam.grpc.interceptors.creator"`
}

// newInterceptorFactory returns the interceptor factory creating the
// interceptors configured at InterceptorConfigPath with the creators of
// the InterceptorCreatorGroup group.
func newInterceptorFactory(
	args interceptorFactoryArgs,
) (flam.IGrpcInterceptorFactory, error) {
	return factory.New(args.Config.Bag(InterceptorConfigPath), args.Creators...)
}

// serverFactoryArgs are the dependencies of the server factory, taken
// from the container. The group tag repeats ServerCreatorGroup.
type serverFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IGrpcServerCreator `group:"flam.grpc.servers.creator"`
}

// newServerFactory returns the server factory creating the servers
// configured at ServerConfigPath with the creators of the
// ServerCreatorGroup group.
func newServerFactory(
	args serverFactoryArgs,
) (flam.IGrpcServerFactory, error) {
	return factory.New(args.Config.Bag(ServerConfigPath), args.Creators...)
}
