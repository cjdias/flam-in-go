// Package standard is the gRPC server driver serving the services of its
// group with the server of [google.golang.org/grpc], over TLS when given a
// certificate: each server listens on its address when run by the kennel,
// serving the services registered by the
// [github.com/cjdias/flam-in-go/pkg/flam.IGrpcServiceRegister] values of
// its group through its interceptors, the first one listed being the
// outermost, and reporting its lifecycle to its logger adaptor, until the
// kennel closes it, letting the calls under way end within its shutdown
// timeout before stopping it at once.
//
// Unless disabled by [ConfigFieldHealth], a server also serves the
// standard health service of
// [google.golang.org/grpc/health/grpc_health_v1], telling the server, and
// each of its services, SERVING while the readiness probe of the
// [github.com/cjdias/flam-in-go/pkg/flam.IHealthRegistry] of the
// container succeeds, run at once and then each [ConfigFieldHealthInterval]
// while the server runs, and NOT_SERVING otherwise, or SERVING as long as
// the server runs when the container provides no registry. Once the
// server stops, they are NOT_SERVING, and the Watch calls end. With
// [ConfigFieldReflection], a server also serves the reflection service of
// [google.golang.org/grpc/reflection], describing its services to the
// clients such as grpcurl.
//
// The services are checked as they are registered: a service registered
// twice, or with an implementation not of its handler type, fails the
// creation of the server with [ErrInvalidService], where the server of
// gRPC would end the process.
package standard
