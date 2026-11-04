package full

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	cacheadaptors "github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors"
	cachekeygenerators "github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators"
	cacheserializers "github.com/cjdias/flam-in-go/pkg/flam/cache/serializers"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	clientbreakers "github.com/cjdias/flam-in-go/pkg/flam/client/breakers"
	clientclients "github.com/cjdias/flam-in-go/pkg/flam/client/clients"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	healthchecks "github.com/cjdias/flam-in-go/pkg/flam/health/checks"
	healthendpoints "github.com/cjdias/flam-in-go/pkg/flam/health/endpoints"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics"
	metricsdatabase "github.com/cjdias/flam-in-go/pkg/flam/metrics/collectors/database"
	metricsredis "github.com/cjdias/flam-in-go/pkg/flam/metrics/collectors/redis"
	metricsendpoints "github.com/cjdias/flam-in-go/pkg/flam/metrics/endpoints"
	metricsmiddleware "github.com/cjdias/flam-in-go/pkg/flam/metrics/middleware"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
	migrationloggers "github.com/cjdias/flam-in-go/pkg/flam/migration/loggers"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	redisconnections "github.com/cjdias/flam-in-go/pkg/flam/redis/connections"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	restloggers "github.com/cjdias/flam-in-go/pkg/flam/rest/loggers"
	restmiddlewares "github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares"
	restservers "github.com/cjdias/flam-in-go/pkg/flam/rest/servers"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	secretssource "github.com/cjdias/flam-in-go/pkg/flam/secrets/source"
	secretsstores "github.com/cjdias/flam-in-go/pkg/flam/secrets/stores"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
	tracingdatabase "github.com/cjdias/flam-in-go/pkg/flam/tracing/database"
	tracingexporters "github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters"
	tracingmiddleware "github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware"
	tracingredis "github.com/cjdias/flam-in-go/pkg/flam/tracing/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
	validatorconverters "github.com/cjdias/flam-in-go/pkg/flam/validator/converters"
	validatorparsers "github.com/cjdias/flam-in-go/pkg/flam/validator/parsers"
	validatorvalidators "github.com/cjdias/flam-in-go/pkg/flam/validator/validators"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	watchdogloggers "github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers"
)

// NewBundle returns the [flam.IBundle] registering the bundles of the
// features [github.com/cjdias/flam-in-go/pkg/flam/app.New] leaves out,
// each followed by the bundle of its drivers, in this order:
//   - [github.com/cjdias/flam-in-go/pkg/flam/database],
//     [github.com/cjdias/flam-in-go/pkg/flam/database/configs] and
//     [github.com/cjdias/flam-in-go/pkg/flam/database/dialects], providing
//     the [flam.IDatabaseConnectionFactory], its config and dialect
//     factories, and their drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/migration] and
//     [github.com/cjdias/flam-in-go/pkg/flam/migration/loggers], providing
//     the [flam.IMigratorFactory], its logger factory, and its drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/redis] and
//     [github.com/cjdias/flam-in-go/pkg/flam/redis/connections], providing
//     the [flam.IRedisConnectionFactory] and its drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/cache],
//     [github.com/cjdias/flam-in-go/pkg/flam/cache/serializers],
//     [github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators] and
//     [github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors], providing the
//     [flam.ICacheAdaptorFactory], its serializer and key generator
//     factories, and their drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/translator] and
//     [github.com/cjdias/flam-in-go/pkg/flam/translator/translators],
//     providing the [flam.ITranslatorFactory] and its drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/validator],
//     [github.com/cjdias/flam-in-go/pkg/flam/validator/parsers] and
//     [github.com/cjdias/flam-in-go/pkg/flam/validator/validators],
//     providing the [flam.IValidatorFactory], its parser and converter
//     factories, and their drivers;
//   - [github.com/cjdias/flam-in-go/pkg/flam/watchdog] and
//     [github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers], providing
//     the [flam.IKennel], its logger adaptor factory, and its drivers,
//     last, as running the kennel blocks while its processes run.
//
// Do not register any of these bundles along with this one, as a bundle
// id can only be registered once.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IBundle
var _ flam.IBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueBundle(database.NewBundle()).
		QueueBundle(configs.NewBundle()).
		QueueBundle(dialects.NewBundle()).
		QueueBundle(migration.NewBundle()).
		QueueBundle(migrationloggers.NewBundle()).
		QueueBundle(redis.NewBundle()).
		QueueBundle(redisconnections.NewBundle()).
		QueueBundle(cache.NewBundle()).
		QueueBundle(cacheserializers.NewBundle()).
		QueueBundle(cachekeygenerators.NewBundle()).
		QueueBundle(cacheadaptors.NewBundle()).
		QueueBundle(translator.NewBundle()).
		QueueBundle(translators.NewBundle()).
		QueueBundle(validator.NewBundle()).
		QueueBundle(validatorparsers.NewBundle()).
		QueueBundle(validatorvalidators.NewBundle()).
		QueueBundle(validatorconverters.NewBundle()).
		QueueBundle(secrets.NewBundle()).
		QueueBundle(secretsstores.NewBundle()).
		QueueBundle(secretssource.NewBundle()).
		QueueBundle(client.NewBundle()).
		QueueBundle(clientbreakers.NewBundle()).
		QueueBundle(clientclients.NewBundle()).
		QueueBundle(broker.NewBundle()).
		QueueBundle(brokers.NewBundle()).
		QueueBundle(tracing.NewBundle()).
		QueueBundle(tracingexporters.NewBundle()).
		QueueBundle(tracingdatabase.NewBundle()).
		QueueBundle(tracingredis.NewBundle()).
		QueueBundle(metrics.NewBundle()).
		QueueBundle(metricsdatabase.NewBundle()).
		QueueBundle(metricsredis.NewBundle()).
		QueueBundle(rest.NewBundle()).
		QueueBundle(restservers.NewBundle()).
		QueueBundle(restloggers.NewBundle()).
		QueueBundle(restmiddlewares.NewBundle()).
		QueueBundle(tracingmiddleware.NewBundle()).
		QueueBundle(metricsmiddleware.NewBundle()).
		QueueBundle(metricsendpoints.NewBundle()).
		QueueBundle(health.NewBundle()).
		QueueBundle(healthchecks.NewBundle()).
		QueueBundle(healthendpoints.NewBundle()).
		QueueBundle(watchdog.NewBundle()).
		QueueBundle(watchdogloggers.NewBundle()).
		Register()
}
