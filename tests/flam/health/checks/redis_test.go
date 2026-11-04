package checks

import (
	"context"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	redischeck "github.com/cjdias/flam-in-go/pkg/flam/health/checks/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// miniValues configures the mini connection "main".
var miniValues = map[string]any{
	redis.ConnectionConfigPath + ".main": container.Bag{factory.ConfigFieldDriver: mini.Driver},
}

// deafConnection is a flam.IRedisConnection whose server never answers.
type deafConnection struct {
	flam.IRedisConnection
}

func (deafConnection) Id() string   { return "deaf" }
func (deafConnection) Close() error { return nil }

func (deafConnection) Ping(ctx context.Context) *goredis.StatusCmd {
	return goredis.NewStatusResult("", errBoom)
}

func Test_Redis_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", redischeck.Driver, "flam.health.checks.driver.redis")
	testutil.Equal(t, "ConfigFieldConnection", redischeck.ConfigFieldConnection, "connection")
	testutil.Equal(t, "ErrorFieldID", redischeck.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", redischeck.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldConnection", redischeck.ErrorFieldConnection, "connection")
}

func Test_Redis_Bundle(t *testing.T) {
	b := redischeck.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks.redis")

	a := newApp(t, nil, b)
	c := creatorOf(t, a)
	testutil.Equal(t, "Accept(driver)", c.Accept(checkBag("main", redischeck.Driver, nil)), true)
	testutil.Equal(t, "Accept(other)", c.Accept(checkBag("main", "other", nil)), false)
}

func Test_Redis_Create(t *testing.T) {
	t.Run("should refuse an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{health.ConfigFieldKind: 2}, health.ConfigFieldKind},
			{container.Bag{redischeck.ConfigFieldConnection: true}, redischeck.ConfigFieldConnection},
			{container.Bag{redischeck.ConfigFieldConnection: ""}, redischeck.ConfigFieldConnection},
		} {
			a := newApp(t, miniValues, redis.NewBundle(), mini.NewBundle(), redischeck.NewBundle())
			check, err := creatorOf(t, a).Create(checkBag("main", redischeck.Driver, tc.fields))
			testutil.Error(t, err, redischeck.ErrInvalidField,
				"unable to create redis health check: invalid redis health check field",
				map[string]any{redischeck.ErrorFieldID: "main", redischeck.ErrorFieldField: tc.field})
			testutil.Equal(t, "check", check, nil)
		}
	})

	t.Run("should fail without connection factory", func(t *testing.T) {
		a := newApp(t, nil, redischeck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("main", redischeck.Driver, nil))
		testutil.Error(t, err, redischeck.ErrNoFactory,
			"unable to create redis health check: no redis connection factory",
			map[string]any{redischeck.ErrorFieldID: "main"})
	})

	t.Run("should fail for an unknown connection", func(t *testing.T) {
		a := newApp(t, miniValues, redis.NewBundle(), mini.NewBundle(), redischeck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("cache", redischeck.Driver, nil))
		testutil.Error(t, err, redischeck.ErrUnknownConnection,
			"unable to create redis health check: unknown redis connection",
			map[string]any{redischeck.ErrorFieldID: "cache", redischeck.ErrorFieldConnection: "cache"})
	})

	t.Run("should check the readiness unless configured", func(t *testing.T) {
		a := newApp(t, miniValues, redis.NewBundle(), mini.NewBundle(), redischeck.NewBundle())
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("main", redischeck.Driver, nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", check.Id(), "main")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)

		check, err = c.Create(checkBag("main", redischeck.Driver, container.Bag{health.ConfigFieldKind: "liveness"}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthLiveness)
	})
}

func Test_Redis_Check(t *testing.T) {
	t.Run("should ping the server of the connection", func(t *testing.T) {
		a := newApp(t, miniValues, redis.NewBundle(), mini.NewBundle(), redischeck.NewBundle())
		defer func() { _ = a.Invoke(func(f flam.IRedisConnectionFactory) error { return f.Close() }) }()

		check, err := creatorOf(t, a).Create(checkBag("cache", redischeck.Driver, container.Bag{
			redischeck.ConfigFieldConnection: "main",
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should fail when the server does not answer", func(t *testing.T) {
		a := newApp(t, nil, redis.NewBundle(), redischeck.NewBundle())
		testutil.NoError(t, a.Invoke(func(f flam.IRedisConnectionFactory) error {
			return f.Store("deaf", deafConnection{})
		}))
		check, err := creatorOf(t, a).Create(checkBag("deaf", redischeck.Driver, nil))
		testutil.NoError(t, err)

		testutil.Error(t, check.Check(context.Background()), redischeck.ErrPing,
			"redis health check failed: unable to ping redis: boom",
			map[string]any{redischeck.ErrorFieldID: "deaf", redischeck.ErrorFieldConnection: "deaf"})
	})

	t.Run("should fail when the connection cannot be created", func(t *testing.T) {
		a := newApp(t, map[string]any{
			redis.ConnectionConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "unknown"},
		}, redis.NewBundle(), redischeck.NewBundle())
		check, err := creatorOf(t, a).Create(checkBag("bad", redischeck.Driver, nil))
		testutil.NoError(t, err)

		wantErr(t, check.Check(context.Background()),
			map[string]any{redischeck.ErrorFieldID: "bad", redischeck.ErrorFieldConnection: "bad"},
			redischeck.ErrConnection, factory.ErrNoCreator)
	})
}
