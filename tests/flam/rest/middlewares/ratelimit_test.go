package middlewares

import (
	"errors"
	"math"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/ratelimit"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// principalSetter is a middleware setting the principal of the requests
// to the value of their X-Principal header, when they hold one.
type principalSetter struct{}

func (principalSetter) Id() string { return "principal" }

func (principalSetter) Wrap(_ flam.RestEndpoint, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if principal := req.Header.Get("X-Principal"); principal != "" {
			req = req.WithContext(rest.WithPrincipal(req.Context(), principal))
		}
		next.ServeHTTP(w, req)
	})
}

// createRatelimit returns the ratelimit middleware "main" of the fields,
// created with the ratelimit bundle, the values and the clock.
func createRatelimit(t *testing.T, fields container.Bag, values map[string]any, clock flam.IClock) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, ratelimit.NewBundle(), ratelimit.Driver, fields, values, deps{clock: clock})
}

// ratelimitFrom returns a request of the remote address, with the headers
// given as name and value pairs.
func ratelimitFrom(remote string, headers ...string) *http.Request {
	req := authRequest("/items/1", headers...)
	req.RemoteAddr = remote
	return req
}

// assertAllowed asserts that the requests are served, in order, through
// the middlewares.
func assertAllowed(t *testing.T, name string, mws []flam.IRestMiddleware, reqs ...*http.Request) {
	t.Helper()
	for _, req := range reqs {
		result := authServe(t, req, mws...)
		testutil.Equal(t, name+" status", result.rec.Code, http.StatusNoContent)
		testutil.Equal(t, name+" served", result.served, true)
	}
}

// assertLimited asserts that the request is answered with a 429 of the
// Retry-After seconds through the middlewares.
func assertLimited(t *testing.T, mws []flam.IRestMiddleware, req *http.Request, retryAfter string) {
	t.Helper()
	assertProblem(t, authServe(t, req, mws...), http.StatusTooManyRequests, "rate limit exceeded", "Retry-After", retryAfter)
}

func Test_Ratelimit_Bundle(t *testing.T) {
	t.Run("should configure the ratelimit middleware", func(t *testing.T) {
		b := ratelimit.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.ratelimit")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + ratelimit.Id: container.Bag{factory.ConfigFieldDriver: ratelimit.Driver},
			ratelimit.DefaultRateConfigPath:                10.0,
			ratelimit.DefaultBurstConfigPath:               20,
			ratelimit.DefaultMaxKeysConfigPath:             10000,
			ratelimit.DefaultTrustedProxyHeaderConfigPath:  "",
			ratelimit.DefaultIPv6PrefixConfigPath:          64,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "Id", ratelimit.Id, "ratelimit")
		testutil.Equal(t, "Driver", ratelimit.Driver, "flam.rest.middlewares.driver.ratelimit")
		for got, want := range map[string]string{
			ratelimit.DefaultRateConfigPath:               "flam.rest.defaults.middlewares.ratelimit.rate",
			ratelimit.DefaultBurstConfigPath:              "flam.rest.defaults.middlewares.ratelimit.burst",
			ratelimit.DefaultMaxKeysConfigPath:            "flam.rest.defaults.middlewares.ratelimit.max_keys",
			ratelimit.DefaultTrustedProxyHeaderConfigPath: "flam.rest.defaults.middlewares.ratelimit.trusted_proxy_header",
			ratelimit.DefaultIPv6PrefixConfigPath:         "flam.rest.defaults.middlewares.ratelimit.ipv6_prefix",
		} {
			testutil.Equal(t, "path", got, want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Ratelimit_Create(t *testing.T) {
	t.Run("should create the middleware of each rate type", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{
			"defaults": {},
			"float":    {ratelimit.ConfigFieldRate: 0.5},
			"int":      {ratelimit.ConfigFieldRate: 3},
			"int64":    {ratelimit.ConfigFieldRate: int64(3)},
			"prefixes": {ratelimit.ConfigFieldIPv6Prefix: 0, ratelimit.ConfigFieldMaxKeys: 1, ratelimit.ConfigFieldBurst: 1},
			"header":   {ratelimit.ConfigFieldTrustedProxyHeader: "X-Forwarded-For", ratelimit.ConfigFieldIPv6Prefix: 128},
		} {
			m, err := createRatelimit(t, fields, nil, testutil.NewClock(epoch))
			if err != nil {
				t.Fatalf("%s: error = %v", name, err)
			}
			testutil.Equal(t, name+" Id()", m.Id(), "main")
		}
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		for name, tc := range map[string]struct {
			fields container.Bag
			values map[string]any
			field  string
		}{
			"rate type":     {container.Bag{ratelimit.ConfigFieldRate: "10"}, nil, ratelimit.ConfigFieldRate},
			"rate zero":     {container.Bag{ratelimit.ConfigFieldRate: 0}, nil, ratelimit.ConfigFieldRate},
			"rate negative": {container.Bag{ratelimit.ConfigFieldRate: -1.5}, nil, ratelimit.ConfigFieldRate},
			"rate infinite": {container.Bag{ratelimit.ConfigFieldRate: math.Inf(1)}, nil, ratelimit.ConfigFieldRate},
			"rate nan":      {container.Bag{ratelimit.ConfigFieldRate: math.NaN()}, nil, ratelimit.ConfigFieldRate},
			"rate default":  {container.Bag{}, map[string]any{ratelimit.DefaultRateConfigPath: int64(0)}, ratelimit.ConfigFieldRate},
			"burst type":    {container.Bag{ratelimit.ConfigFieldBurst: 1.0}, nil, ratelimit.ConfigFieldBurst},
			"burst zero":    {container.Bag{ratelimit.ConfigFieldBurst: 0}, nil, ratelimit.ConfigFieldBurst},
			"max keys type": {container.Bag{ratelimit.ConfigFieldMaxKeys: "1"}, nil, ratelimit.ConfigFieldMaxKeys},
			"max keys zero": {container.Bag{ratelimit.ConfigFieldMaxKeys: 0}, nil, ratelimit.ConfigFieldMaxKeys},
			"header":        {container.Bag{ratelimit.ConfigFieldTrustedProxyHeader: true}, nil, ratelimit.ConfigFieldTrustedProxyHeader},
			"prefix type":   {container.Bag{ratelimit.ConfigFieldIPv6Prefix: "64"}, nil, ratelimit.ConfigFieldIPv6Prefix},
			"prefix low":    {container.Bag{ratelimit.ConfigFieldIPv6Prefix: -1}, nil, ratelimit.ConfigFieldIPv6Prefix},
			"prefix high":   {container.Bag{ratelimit.ConfigFieldIPv6Prefix: 129}, nil, ratelimit.ConfigFieldIPv6Prefix},
		} {
			_, err := createRatelimit(t, tc.fields, tc.values, testutil.NewClock(epoch))
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			testutil.Error(t, errors.Unwrap(err), ratelimit.ErrInvalidField,
				"unable to create ratelimit rest middleware: invalid ratelimit rest middleware field",
				map[string]any{ratelimit.ErrorFieldID: "main", ratelimit.ErrorFieldField: tc.field})
		}
	})
}

func Test_Ratelimit(t *testing.T) {
	limiter := func(t *testing.T, fields container.Bag, clock flam.IClock) []flam.IRestMiddleware {
		t.Helper()
		m, err := createRatelimit(t, fields, nil, clock)
		testutil.NoError(t, err)
		return []flam.IRestMiddleware{principalSetter{}, m}
	}

	t.Run("should serve the burst, then answer with a 429 until a token is back", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 0.25, ratelimit.ConfigFieldBurst: 2}, clock)
		from := func() *http.Request { return ratelimitFrom("10.0.0.1:4321") }

		assertAllowed(t, "burst", mws, from(), from())
		assertLimited(t, mws, from(), "4")
		clock.Advance(time.Second)
		assertLimited(t, mws, from(), "3")
		clock.Advance(2500 * time.Millisecond)
		assertLimited(t, mws, from(), "1")
		clock.Advance(500 * time.Millisecond)
		assertAllowed(t, "refilled", mws, from())
		assertLimited(t, mws, from(), "4")
	})

	t.Run("should round the Retry-After up to a whole second", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 10, ratelimit.ConfigFieldBurst: 1}, clock)
		assertAllowed(t, "first", mws, ratelimitFrom("10.0.0.1:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "1")
		clock.Advance(100 * time.Millisecond)
		assertAllowed(t, "refilled", mws, ratelimitFrom("10.0.0.1:1"))
	})

	t.Run("should cap the Retry-After to a year", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1e-300, ratelimit.ConfigFieldBurst: 1}, clock)
		assertAllowed(t, "first", mws, ratelimitFrom("10.0.0.1:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "31536000")
		clock.Advance(1000 * time.Hour)
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "31536000")
	})

	t.Run("should limit each client apart", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1}, clock)

		assertAllowed(t, "first ip", mws, ratelimitFrom("10.0.0.1:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:2"), "1")
		assertLimited(t, mws, ratelimitFrom("[::ffff:10.0.0.1]:3"), "1")
		assertAllowed(t, "other ip", mws, ratelimitFrom("10.0.0.2:1"))
		assertAllowed(t, "principal", mws, ratelimitFrom("10.0.0.1:1", "X-Principal", "alice"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.3:1", "X-Principal", "alice"), "1")
		assertAllowed(t, "other principal", mws, ratelimitFrom("10.0.0.1:1", "X-Principal", "bob"))
		assertAllowed(t, "principal named as an ip key", mws, ratelimitFrom("10.0.0.9:1", "X-Principal", "10.0.0.2"))
	})

	t.Run("should limit the remote addresses without port or ip", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1}, clock)

		assertAllowed(t, "no port", mws, ratelimitFrom("10.0.0.1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:80"), "1")
		assertAllowed(t, "not an ip", mws, ratelimitFrom("@pipe"))
		assertLimited(t, mws, ratelimitFrom("@pipe"), "1")
		assertAllowed(t, "empty", mws, ratelimitFrom(""))
		assertLimited(t, mws, ratelimitFrom(""), "1")
	})

	t.Run("should limit an ipv6 client by its prefix", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1}, clock)
		assertAllowed(t, "first", mws, ratelimitFrom("[2001:db8:1:2::1]:1"))
		assertLimited(t, mws, ratelimitFrom("[2001:db8:1:2:ffff::9]:1"), "1")
		assertLimited(t, mws, ratelimitFrom("[2001:db8:1:2::1%eth0]:1"), "1")
		assertAllowed(t, "other prefix", mws, ratelimitFrom("[2001:db8:1:3::1]:1"))

		mws = limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1, ratelimit.ConfigFieldIPv6Prefix: 128}, clock)
		assertAllowed(t, "full", mws, ratelimitFrom("[2001:db8:1:2::1]:1"), ratelimitFrom("[2001:db8:1:2::2]:1"))
		assertLimited(t, mws, ratelimitFrom("[2001:db8:1:2::1]:2"), "1")
	})

	t.Run("should ignore the proxy header unless trusted", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1}, clock)
		assertAllowed(t, "first", mws, ratelimitFrom("10.0.0.1:1", "X-Forwarded-For", "1.1.1.1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1", "X-Forwarded-For", "2.2.2.2"), "1")
	})

	t.Run("should take the last ip of the trusted proxy header", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1, ratelimit.ConfigFieldTrustedProxyHeader: "X-Forwarded-For"}, clock)
		proxy := "192.168.0.1:1"

		assertAllowed(t, "client", mws, ratelimitFrom(proxy, "X-Forwarded-For", "1.1.1.1"))
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "1.1.1.1"), "1")
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "9.9.9.9, 1.1.1.1"), "1")
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "9.9.9.9", "X-Forwarded-For", " 1.1.1.1 "), "1")
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "1.1.1.1:8080"), "1")
		assertAllowed(t, "spoofed first", mws, ratelimitFrom(proxy, "X-Forwarded-For", "1.1.1.1, 2.2.2.2"))

		assertAllowed(t, "no header", mws, ratelimitFrom(proxy))
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "garbage"), "1")
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "1.1.1.1, "), "1")
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", ""), "1")

		assertAllowed(t, "ipv6", mws, ratelimitFrom(proxy, "X-Forwarded-For", "2001:db8::1"))
		assertLimited(t, mws, ratelimitFrom(proxy, "X-Forwarded-For", "[2001:db8::2]:443"), "1")
	})

	t.Run("should drop the least recently seen bucket over the maximum", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1e-3, ratelimit.ConfigFieldBurst: 1, ratelimit.ConfigFieldMaxKeys: 2}, clock)

		assertAllowed(t, "a b", mws, ratelimitFrom("10.0.0.1:1"), ratelimitFrom("10.0.0.2:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "1000")
		assertAllowed(t, "c drops b", mws, ratelimitFrom("10.0.0.3:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "1000")
		assertAllowed(t, "b again drops c", mws, ratelimitFrom("10.0.0.2:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "1000")
		assertAllowed(t, "c again", mws, ratelimitFrom("10.0.0.3:1"))
	})

	t.Run("should drop the buckets full again without losing their state", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 2, ratelimit.ConfigFieldMaxKeys: 2}, clock)

		assertAllowed(t, "a", mws, ratelimitFrom("10.0.0.1:1"), ratelimitFrom("10.0.0.1:1"))
		assertAllowed(t, "b", mws, ratelimitFrom("10.0.0.2:1"))
		clock.Advance(1999 * time.Millisecond)
		assertAllowed(t, "a not full", mws, ratelimitFrom("10.0.0.1:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "1")
		clock.Advance(2 * time.Second)
		assertAllowed(t, "c", mws, ratelimitFrom("10.0.0.3:1"))
		assertAllowed(t, "a full", mws, ratelimitFrom("10.0.0.1:1"), ratelimitFrom("10.0.0.1:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.1:1"), "1")
		assertAllowed(t, "c kept", mws, ratelimitFrom("10.0.0.3:1"))
		assertLimited(t, mws, ratelimitFrom("10.0.0.3:1"), "1")
	})

	t.Run("should limit the requests matching no endpoint", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 1}, clock)
		result := authServe(t, authRequest("/unknown"), mws...)
		testutil.Equal(t, "status", result.rec.Code, http.StatusNotFound)
		assertLimited(t, mws, authRequest("/items/1"), "1")
	})

	t.Run("should take each token once under concurrent use", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		mws := limiter(t, container.Bag{ratelimit.ConfigFieldRate: 1, ratelimit.ConfigFieldBurst: 10}, clock)
		var wg sync.WaitGroup
		var mu sync.Mutex
		served := map[string]int{}
		for _, remote := range []string{"10.0.0.1:1", "10.0.0.2:1", "10.0.0.3:1", "10.0.0.4:1"} {
			for range 15 {
				wg.Go(func() {
					result := authServe(t, ratelimitFrom(remote), mws...)
					mu.Lock()
					defer mu.Unlock()
					served[result.rec.Header().Get("Retry-After")]++
				})
			}
		}
		wg.Wait()
		testutil.Equal(t, "served", served[""], 40)
		testutil.Equal(t, "limited", served["1"], 20)
	})
}
