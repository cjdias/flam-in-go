// Package grpc is the gRPC feature: the bundle returned from [NewBundle]
// provides the [github.com/cjdias/flam-in-go/pkg/flam.IGrpcServerFactory]
// of an application, along with the factories of the logger adaptors and
// of the interceptors of its servers, and adds its servers to the kennel
// of [github.com/cjdias/flam-in-go/pkg/flam/watchdog], which runs them,
// and stops them on an interrupt or a SIGTERM signal.
//
// Each server serves the services of its group, registered by the
// [github.com/cjdias/flam-in-go/pkg/flam.IGrpcServiceRegister] values the
// application provides in the [ServiceGroup] container group, such as:
//
//	func (r *users) Group() string { return "api" }
//
//	func (r *users) Register(registrar grpc.ServiceRegistrar) error {
//		pb.RegisterUsersServer(registrar, r)
//		return nil
//	}
//
// configured, with the drivers of the sub packages registered, as:
//
//	flam:
//	  grpc:
//	    servers:
//	      api: {driver: flam.grpc.servers.driver.standard, address: ":9090"}
//
// The interceptors receive the full method of the calls, such as
// "/pkg.Users/Get", which [SplitMethod] splits into its service and its
// method, and [IsServerError] tells the codes of the statuses telling a
// failure of the server rather than of the call.
//
// As a process of the kennel, a server also reports its start and its end
// to its watchdog logger adaptor, configured at
// [watchdog.ProcessConfigPath] by its id, along with its own logger
// adaptor, which reports its address, its errors and its shutdown too. To
// log them once, give the server no watchdog logger adaptor:
//
//	flam:
//	  watchdog:
//	    processes:
//	      api: {logger: ""}
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/grpc/servers],
// [github.com/cjdias/flam-in-go/pkg/flam/grpc/loggers] and
// [github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors]. They import
// this package for its paths and groups, so this package never imports
// them.
package grpc
