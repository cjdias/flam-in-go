// Package flam is the root of flam, a Go SDK made of reusable
// functionalities. It holds the interfaces through which the
// functionalities are used, and the types they share:
//   - Error is an error with a message, an optional cause and a context of
//     key/value fields, built with NewError and NewWrappedError and refined
//     with Error.With;
//   - IBag is the interface of a tree of loosely typed values addressed by
//     dot separated paths, such as "server.port", implemented by Bag in
//     [github.com/cjdias/flam-in-go/pkg/flam/container];
//   - IFactory is the interface of a factory creating, holding and releasing
//     resources, which implement IResource, from their configuration with
//     IResourceCreator values, implemented by the factory returned from
//     New in [github.com/cjdias/flam-in-go/pkg/flam/factory];
//   - IDisk is the interface of a file system held by the IDiskFactory, the
//     IFactory of disks, whose drivers provide IDiskCreator values,
//     implemented in [github.com/cjdias/flam-in-go/pkg/flam/disk];
//   - IPubSub is the interface of an in-process publish/subscribe hub,
//     implemented by PubSub in [github.com/cjdias/flam-in-go/pkg/flam/pubsub];
//   - IClock is the interface of the time source of a program, reading the
//     current time, building and parsing times, and making the ITimer and
//     ITicker timers, so that tests can fake it, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/clock];
//   - ITrigger is the interface of a timer calling a callback once or
//     periodically, created and tracked by an ITriggerFactory reading the
//     time from an IClock, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/trigger];
//   - IConfig is the interface of the shared configuration of an
//     application, observable by path, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/config];
//   - IApp is the interface of an application wiring the features of a
//     program, its bundles implementing IBundle, into a dependency
//     injection container, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/app], and IRegisterer helps
//     the bundles register in it, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/bundle];
//   - IDatabaseConnection is the interface of a gorm database connection,
//     held by an IDatabaseConnectionFactory, narrowed by an IRequest,
//     implemented by Request in
//     [github.com/cjdias/flam-in-go/pkg/flam/database], and Record is a
//     base model publishing its lifecycle events as RecordEvent values;
//   - IMigrator applies and reverts IMigration values to a database,
//     implemented in [github.com/cjdias/flam-in-go/pkg/flam/migration];
//   - IRedisConnection is the interface of a Redis connection, held by an
//     IRedisConnectionFactory, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/redis];
//   - ICacheAdaptor caches objects under the keys of an
//     ICacheKeyGenerator as the strings of an ICacheSerializer,
//     implemented in [github.com/cjdias/flam-in-go/pkg/flam/cache];
//   - ITranslator translates the keys of its texts into the language of
//     its locale, held by an ITranslatorFactory, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/translator];
//   - IValidator validates the structs, describing their failures as
//     ValidationError values with an IValidationParser and converting
//     them with an IValidationErrorConverter, each held by its factory,
//     implemented in [github.com/cjdias/flam-in-go/pkg/flam/validator];
//   - IKennel holds the long running processes of an application,
//     implementing IWatchdogProcess, and runs them under watchdogs
//     reporting to an IWatchdogLoggerAdaptor, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/watchdog];
//   - IRestServer serves the endpoints registered by the
//     IRestEndpointRegister values of its group on its IRestRouter,
//     through IRestMiddleware values, reporting to an
//     IRestLoggerAdaptor, each held by its factory, run as a process of
//     the IKennel, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/rest];
//   - IClient sends HTTP requests, retrying them and failing fast through
//     an IClientCircuitBreaker, each held by its factory, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/client];
//   - ICommand is a command line command run by the application instead
//     of its processes, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/command];
//   - IHealthCheck checks a dependency of the application for the
//     liveness and readiness probes of an IHealthRegistry, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/health];
//   - ITracingExporter exports the spans of the OpenTelemetry tracer
//     provider of the application, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/tracing], while the metrics
//     of [github.com/cjdias/flam-in-go/pkg/flam/metrics] are Prometheus
//     collectors;
//   - IBroker publishes messages to, and subscribes to, the topics of a
//     message broker, such as Redis, NATS or Kafka, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/broker];
//   - ISecretStore reads secrets, such as from Vault, held by an
//     ISecretStoreFactory, implemented in
//     [github.com/cjdias/flam-in-go/pkg/flam/secrets].
//
// This package holds every contract of flam, and the implementations live
// in sub packages importing this one, so that code can depend on the
// contracts only. As IApp exposes its [go.uber.org/dig] container, this
// package depends on dig.
package flam
