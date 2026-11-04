package flam

import (
	"google.golang.org/grpc"
)

// IGrpcServiceRegister registers services on the gRPC servers of its
// group.
type IGrpcServiceRegister interface {
	// Group returns the group of the servers the register registers its
	// services on.
	Group() string

	// Register registers the services on a server, such as with the
	// RegisterXxxServer function generated for a service. The registrar
	// must not be kept, as no service can be registered once the server is
	// created.
	Register(registrar grpc.ServiceRegistrar) error
}

// IGrpcInterceptor intercepts the calls served by a gRPC server, such as
// to log them. It is held by an [IGrpcInterceptorFactory].
type IGrpcInterceptor interface {
	IResource

	// Unary returns the interceptor of the unary calls, or nil to leave
	// them alone.
	Unary() grpc.UnaryServerInterceptor

	// Stream returns the interceptor of the streaming calls, or nil to
	// leave them alone.
	Stream() grpc.StreamServerInterceptor
}

// IGrpcInterceptorCreator creates the gRPC interceptors of a driver, for
// the [IGrpcInterceptorFactory].
type IGrpcInterceptorCreator = IResourceCreator[IGrpcInterceptor]

// IGrpcInterceptorFactory creates and holds the gRPC interceptors, one per
// id.
type IGrpcInterceptorFactory = IFactory[IGrpcInterceptor]

// IGrpcLoggerAdaptor reports the lifecycle of the gRPC servers, each by
// its id: its start, with the address it listens on, its failures, its
// shutdown and its end. It is held by an [IGrpcLoggerAdaptorFactory].
type IGrpcLoggerAdaptor interface {
	IResource

	Start(id string, address string)
	Error(id string, e error)
	Shutdown(id string)
	Done(id string)
}

// IGrpcLoggerAdaptorCreator creates the gRPC logger adaptors of a driver,
// for the [IGrpcLoggerAdaptorFactory].
type IGrpcLoggerAdaptorCreator = IResourceCreator[IGrpcLoggerAdaptor]

// IGrpcLoggerAdaptorFactory creates and holds the gRPC logger adaptors, one
// per id.
type IGrpcLoggerAdaptorFactory = IFactory[IGrpcLoggerAdaptor]

// IGrpcServer is a gRPC server, run as a process of the [IKennel]: Run
// serves its services until its context is done or it is closed, then
// stops it gracefully. It is held by an [IGrpcServerFactory].
type IGrpcServer interface {
	IWatchdogProcess

	// Address returns the address the server listens on while it runs,
	// such as "127.0.0.1:9090", or the address it is configured with
	// otherwise.
	Address() string
}

// IGrpcServerCreator creates the gRPC servers of a driver, for the
// [IGrpcServerFactory].
type IGrpcServerCreator = IResourceCreator[IGrpcServer]

// IGrpcServerFactory creates and holds the gRPC servers, one per id.
type IGrpcServerFactory = IFactory[IGrpcServer]
