package full

import (
	"context"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/app/full"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	parserstandard "github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
	validatorstandard "github.com/cjdias/flam-in-go/pkg/flam/validator/validators/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// bundleIds are the ids of the bundles registered by full.NewBundle, in
// registration order.
var bundleIds = []string{
	"flam.database", "flam.database.configs", "flam.database.configs.standard",
	"flam.database.dialects", "flam.database.dialects.sqlite", "flam.database.dialects.mysql", "flam.database.dialects.postgres",
	"flam.migration", "flam.migration.loggers", "flam.migration.loggers.standard",
	"flam.redis", "flam.redis.connections", "flam.redis.connections.standard", "flam.redis.connections.mini",
	"flam.cache",
	"flam.cache.serializers", "flam.cache.serializers.json",
	"flam.cache.key_generators", "flam.cache.key_generators.field",
	"flam.cache.adaptors", "flam.cache.adaptors.redis", "flam.cache.adaptors.memory",
	"flam.translator", "flam.translator.translators",
	"flam.translator.translators.english", "flam.translator.translators.portuguese", "flam.translator.translators.spanish",
	"flam.translator.translators.french", "flam.translator.translators.german",
	"flam.validator", "flam.validator.parsers", "flam.validator.parsers.standard",
	"flam.validator.validators", "flam.validator.validators.standard",
	"flam.validator.converters", "flam.validator.converters.envelope",
	"flam.secrets", "flam.secrets.stores", "flam.secrets.stores.env", "flam.secrets.stores.file", "flam.secrets.stores.vault",
	"flam.secrets.source",
	"flam.client", "flam.client.breakers", "flam.client.breakers.standard", "flam.client.clients", "flam.client.clients.standard",
	"flam.broker", "flam.broker.brokers", "flam.broker.brokers.redis", "flam.broker.brokers.nats", "flam.broker.brokers.kafka",
	"flam.tracing", "flam.tracing.exporters", "flam.tracing.exporters.none", "flam.tracing.exporters.otlphttp",
	"flam.tracing.database", "flam.tracing.redis",
	"flam.metrics", "flam.metrics.collectors.database", "flam.metrics.collectors.redis",
	"flam.rest", "flam.rest.servers", "flam.rest.servers.standard",
	"flam.rest.loggers", "flam.rest.loggers.standard",
	"flam.rest.middlewares", "flam.rest.middlewares.envelope", "flam.rest.middlewares.log",
	"flam.rest.middlewares.jwt", "flam.rest.middlewares.apikey", "flam.rest.middlewares.ratelimit",
	"flam.rest.middlewares.requestid", "flam.rest.middlewares.cors", "flam.rest.middlewares.bodylimit", "flam.rest.middlewares.timeout",
	"flam.rest.middlewares.tracing", "flam.metrics.middleware", "flam.metrics.endpoints",
	"flam.health", "flam.health.checks", "flam.health.checks.database", "flam.health.checks.redis", "flam.health.checks.kennel",
	"flam.health.endpoints",
	"flam.watchdog", "flam.watchdog.loggers", "flam.watchdog.loggers.standard",
}

// job is a watchdog process ending at once.
type job struct{}

func (job) Id() string                { return "job" }
func (job) Run(context.Context) error { return nil }
func (job) IsRunning() bool           { return false }
func (job) Close() error              { return nil }

// item is the object cached and validated by the tests.
type item struct {
	Id   int
	Name string `validate:"required"`
}

func Test_Bundle(t *testing.T) {
	t.Run("should register every opt-in feature bundle", func(t *testing.T) {
		b := full.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.full")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		var registered []string
		for _, call := range a.Calls() {
			if len(call) > len("bundle:") && call[:len("bundle:")] == "bundle:" {
				registered = append(registered, call[len("bundle:"):])
			}
		}
		testutil.Equal(t, "bundles", registered, append([]string{"flam.full"}, bundleIds...))
	})

	t.Run("should be refused by an application registering its bundles already", func(t *testing.T) {
		a := app.New(nil)
		testutil.NoError(t, a.RegisterBundle(database.NewBundle()))
		if err := a.RegisterBundle(full.NewBundle()); err == nil {
			t.Fatal("RegisterBundle() did not fail")
		}
	})
}

func Test_New(t *testing.T) {
	t.Run("should register the core and opt-in bundles", func(t *testing.T) {
		a := full.New(nil)
		for _, id := range append([]string{"flam.config", "flam.log", "flam.full"}, bundleIds...) {
			testutil.Equal(t, "HasBundle("+id+")", a.HasBundle(id), true)
		}
		testutil.NoError(t, a.Run())
		testutil.NoError(t, a.Close())
	})

	t.Run("should use every feature", func(t *testing.T) {
		values := container.Bag{}
		for path, value := range map[string]any{
			database.DialectConfigPath + ".main":       container.Bag{factory.ConfigFieldDriver: sqlite.Driver, sqlite.ConfigFieldPath: ":memory:"},
			database.ConnectionConfigPath + ".main":    container.Bag{database.ConfigFieldDialect: "main"},
			migration.MigratorConfigPath + ".main":     container.Bag{migration.ConfigFieldConnection: "main"},
			redis.ConnectionConfigPath + ".main":       container.Bag{factory.ConfigFieldDriver: mini.Driver},
			cache.AdaptorConfigPath + ".main":          container.Bag{factory.ConfigFieldDriver: "flam.cache.adaptors.driver.redis", "connection": "main"},
			translator.ConfigPath + ".en.translations": container.Bag{"welcome": "Welcome, {0}!"},
			parserstandard.DefaultTranslatorConfigPath: "en",
		} {
			testutil.NoError(t, values.Set(path, value))
		}
		a := full.New(values)
		testutil.NoError(t, a.Boot())
		testutil.NoError(t, a.Invoke(func(
			migrators flam.IMigratorFactory, adaptors flam.ICacheAdaptorFactory, translators flam.ITranslatorFactory,
			validators flam.IValidatorFactory, kennel flam.IKennel,
		) error {
			m, err := migrators.Get("main")
			if err != nil {
				return err
			}
			if _, err := m.List(); err != nil {
				return err
			}
			adaptor, err := adaptors.Get("main")
			if err != nil {
				return err
			}
			if err := adaptor.Set(context.Background(), &item{Id: 1}); err != nil {
				return err
			}
			tr, err := translators.Get("en")
			if err != nil {
				return err
			}
			got, err := tr.T("welcome", "Ann")
			testutil.Equal(t, "T()", got, "Welcome, Ann!")
			if err != nil {
				return err
			}
			v, err := validators.Get(validatorstandard.Id)
			if err != nil {
				return err
			}
			testutil.Equal(t, "Validate()", v.Validate(&item{Id: 1}), any([]flam.ValidationError{
				{ParamName: "Name", ErrorId: 104, ErrorMessage: "Name is a required field"},
			}))
			return kennel.Add("job", true, "standard", job{})
		}))
		testutil.NoError(t, a.Close())
	})
}
