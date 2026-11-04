# flam

flam is a Go SDK of reusable building blocks for services: configuration,
logging, databases, caches, messaging, REST servers and their operations,
wired together through a dependency injection container
([uber-go/dig](https://github.com/uber-go/dig)).

Every feature is a **bundle** registered on an application. Its resources
are created by **factories** from the configuration, through **drivers**
living in their own packages, so a program links only what it registers.
The interfaces of every feature live in the root package
[`pkg/flam`](pkg/flam), so code can depend on the contracts alone.

```
go get github.com/cjdias/flam-in-go
```

flam requires Go 1.26 or later; build with the latest patch release, as it
fixes vulnerabilities of the standard library.

## Quick start

A REST server, run under a watchdog and shut down gracefully on SIGINT or
SIGTERM:

```go
package main

import (
	"log"
	"net/http"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app/full"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/servers/standard"
)

// hello registers the endpoints of the "api" servers.
type hello struct{}

func (hello) Group() string { return "api" }

func (hello) Register(router flam.IRestRouter) error {
	return router.Handle(
		flam.RestEndpoint{Id: 1, Pattern: "GET /hello"},
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope("hello"))
		}))
}

func main() {
	a := full.New(container.Bag{
		"flam": container.Bag{
			"rest": container.Bag{
				"service_id": 1,
				"servers": container.Bag{
					"api": container.Bag{
						"driver":      standard.Driver,
						"address":     ":8080",
						"middlewares": []any{"requestid", "cors", "log", "envelope"},
					},
				},
			},
		},
	})
	defer func() { _ = a.Close() }()

	if err := a.Provide(func() flam.IRestEndpointRegister { return hello{} },
		dig.Group(rest.EndpointGroup)); err != nil {
		log.Fatal(err)
	}
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
```

`GET /hello` answers `{"data":"hello"}`, while an unknown route answers an
RFC 9457 `application/problem+json` error. In a real program the
configuration usually comes from files and environment variables, through
the sources of [`pkg/flam/config`](pkg/flam/config).

[`app.New`](pkg/flam/app) registers only the core bundles: configuration,
logging, clock, triggers, disks and pub/sub. [`full.New`](pkg/flam/app/full)
registers every feature but the commands; register only the bundles you
need on `app.New` to keep the binary small.

## Features

| Feature | Package | Drivers |
|---|---|---|
| Configuration, observable and reloadable | [`config`](pkg/flam/config) | sources: env, file, dir, rest, Consul, etcd; parsers: json, yaml |
| Logging, context-aware, with a `log/slog` bridge | [`log`](pkg/flam/log), [`slogbridge`](pkg/flam/log/slogbridge) | streams: console, file, rotating file, syslog, OTLP; serializers: json, stringify; trace, request and principal ids from the context |
| Clock, triggers and cron | [`clock`](pkg/flam/clock), [`trigger`](pkg/flam/trigger), [`scheduler`](pkg/flam/scheduler) | cron specs with time zones; scheduled jobs, optionally single across instances |
| Disks (`afero.Fs`) | [`disk`](pkg/flam/disk) | local, memory, S3, GCS |
| In-process pub/sub | [`pubsub`](pkg/flam/pubsub) | |
| Databases (gorm) and migrations | [`database`](pkg/flam/database), [`migration`](pkg/flam/migration) | sqlite, mysql, postgres; read replicas, transient error retries |
| MongoDB | [`mongo`](pkg/flam/mongo) | standard |
| Redis | [`redis`](pkg/flam/redis) | standard, mini (miniredis, for tests) |
| Cache | [`cache`](pkg/flam/cache) | adaptors: redis, memory, invalidating (across instances through a broker); cache-aside `Loader` with single-flight loads |
| Distributed locks and leader election | [`lock`](pkg/flam/lock) | redis, memory; fencing tokens |
| Message brokers | [`broker`](pkg/flam/broker) | redis (pub/sub, streams), NATS (core, JetStream), Kafka; traced and measured |
| Background jobs and outbox | [`jobs`](pkg/flam/jobs), [`outbox`](pkg/flam/outbox) | database queue with retries and dead letters; transactional outbox relayed to a broker |
| Translations | [`translator`](pkg/flam/translator) | en, pt, es, fr, de |
| Validation | [`validator`](pkg/flam/validator) | go-playground validator, envelope converter |
| Secrets | [`secrets`](pkg/flam/secrets) | env, file, Vault, AWS Secrets Manager, GCP Secret Manager; a config source reading them |
| HTTP client with circuit breaker | [`client`](pkg/flam/client) | standard client and breaker; traced and measured |
| REST servers | [`rest`](pkg/flam/rest), [`envelope`](pkg/flam/envelope), [`openapi`](pkg/flam/rest/openapi) | HTTP/HTTPS server; middlewares below; OpenAPI 3.1 document and Swagger UI; Server-Sent Events |
| gRPC servers | [`grpc`](pkg/flam/grpc) | standard server with health and reflection; interceptors: recovery, log, tracing, metrics |
| Long running processes | [`watchdog`](pkg/flam/watchdog) | graceful drain on SIGTERM (`drain_delay`) |
| Health probes | [`health`](pkg/flam/health) | checks: database, redis, kennel (fails while draining), broker, http, disk; `/health/live`, `/health/ready` |
| Metrics (Prometheus) | [`metrics`](pkg/flam/metrics) | REST middleware, `/metrics`, database and redis collectors |
| Tracing (OpenTelemetry) | [`tracing`](pkg/flam/tracing) | exporters: OTLP/HTTP, none; REST middleware, gorm plugin, redis hook |
| Command line commands | [`command`](pkg/flam/command) | `help`, `migrate` |

### REST middlewares

Middlewares are configured by id and listed per server, the first being the
outermost. The recommended order is
`requestid, cors, secure, etag, compress, log, envelope, jwt|apikey, authz, ratelimit, idempotency, bodylimit, timeout`,
with `tracing` and `metrics` near the outside. Endpoints marked `Streaming`
bypass the buffering middlewares.

| Middleware | Purpose |
|---|---|
| `requestid` | reads or generates the request id, put in the context and the logs |
| `cors` | CORS headers and preflight answers |
| `log` | logs requests and responses, redacting headers and query parameters |
| `envelope` | answers panics with envelope errors |
| `secure` | security headers, such as HSTS and `X-Content-Type-Options` |
| `etag`, `compress` | ETags with 304 answers, and gzip/deflate compression |
| `jwt`, `apikey` | authentication, setting the principal of the request; JWT keys from a JWKS or OIDC discovery |
| `authz` | scopes and roles required per endpoint |
| `idempotency` | replays the response of a repeated `Idempotency-Key` |
| `ratelimit` | token bucket per principal or client IP |
| `bodylimit` | caps the size of the request bodies |
| `timeout` | answers a 503 when a handler runs too long |
| `tracing`, `metrics` | OpenTelemetry spans and Prometheus metrics |

## Configuration

Everything is configured under the `flam` path, each factory reading one
configuration per resource id, its `driver` field naming the driver that
creates it, such as:

```yaml
flam:
  database:
    dialects:
      pg: {driver: flam.database.dialects.driver.postgres, host: db, database: app}
    connections:
      main: {driver: flam.database.connections.driver.gorm, dialect: pg}
  rest:
    servers:
      api: {driver: flam.rest.servers.driver.standard, address: ":8080"}
  watchdog:
    processes:
      api: {logger: ""}
```

The defaults of the fields of each driver live under the `defaults` path of
its feature, such as `flam.rest.defaults.servers.standard.address`, and the
package documentation of each driver lists its fields.

## Development

The project uses [Task](https://taskfile.dev) and pins its tools in
[`tools/go.mod`](tools/go.mod):

```
task check   # gofmt, go vet, golangci-lint, staticcheck, govulncheck, tests and coverage
task test    # tests with the race detector
task cover   # tests, failing below 100% statement coverage
```

The tests live in [`tests/flam`](tests/flam), mirroring the packages, and
cover every statement of [`pkg`](pkg). The runnable examples live next to
their packages, in `example_test.go` files.

## License

[MIT](LICENSE)
