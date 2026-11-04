package checks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/client/clients/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	httpcheck "github.com/cjdias/flam-in-go/pkg/flam/health/checks/http"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// httpServer returns a server answering "/" and "/ok" with 200, "/down"
// with 503, and "/slow" once the request is cancelled, closed once the
// test ends.
func httpServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/down", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	mux.HandleFunc("/slow", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// httpApp returns an app with the client factory, the standard client
// "main" of the base URL, the values, and the http check driver.
func httpApp(t *testing.T, baseURL string, values map[string]any) *testutil.App {
	t.Helper()
	all := map[string]any{
		client.ClientConfigPath + ".main": container.Bag{
			factory.ConfigFieldDriver:   standard.Driver,
			standard.ConfigFieldBaseURL: baseURL,
			standard.ConfigFieldRetries: 0,
			standard.ConfigFieldTracing: false,
		},
	}
	for path, value := range values {
		all[path] = value
	}
	a := newApp(t, all, client.NewBundle(), standard.NewBundle(), httpcheck.NewBundle())
	t.Cleanup(func() {
		_ = a.Invoke(func(f flam.IClientFactory) error { return f.Close() })
	})
	return a
}

func Test_HTTP_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", httpcheck.Driver, "flam.health.checks.driver.http")
	testutil.Equal(t, "ConfigFieldURL", httpcheck.ConfigFieldURL, "url")
	testutil.Equal(t, "ConfigFieldClient", httpcheck.ConfigFieldClient, "client")
	testutil.Equal(t, "ConfigFieldMinStatus", httpcheck.ConfigFieldMinStatus, "min_status")
	testutil.Equal(t, "ConfigFieldMaxStatus", httpcheck.ConfigFieldMaxStatus, "max_status")
	testutil.Equal(t, "ConfigFieldTimeout", httpcheck.ConfigFieldTimeout, "timeout")
	testutil.Equal(t, "ErrorFieldID", httpcheck.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", httpcheck.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldClient", httpcheck.ErrorFieldClient, "client")
	testutil.Equal(t, "ErrorFieldURL", httpcheck.ErrorFieldURL, "url")
	testutil.Equal(t, "ErrorFieldStatus", httpcheck.ErrorFieldStatus, "status")
}

func Test_HTTP_Bundle(t *testing.T) {
	b := httpcheck.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks.http")

	a := newApp(t, nil, b)
	c := creatorOf(t, a)
	testutil.Equal(t, "Accept(driver)", c.Accept(checkBag("search", httpcheck.Driver, nil)), true)
	testutil.Equal(t, "Accept(other)", c.Accept(checkBag("search", "other", nil)), false)
}

func Test_HTTP_Create(t *testing.T) {
	const target = "http://localhost/health"

	t.Run("should refuse an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{health.ConfigFieldKind: "none", httpcheck.ConfigFieldURL: target}, health.ConfigFieldKind},
			{container.Bag{httpcheck.ConfigFieldClient: 1, httpcheck.ConfigFieldURL: target}, httpcheck.ConfigFieldClient},
			{container.Bag{httpcheck.ConfigFieldURL: 1}, httpcheck.ConfigFieldURL},
			{container.Bag{}, httpcheck.ConfigFieldURL},
			{container.Bag{httpcheck.ConfigFieldURL: "/health"}, httpcheck.ConfigFieldURL},
			{container.Bag{httpcheck.ConfigFieldURL: "ftp://localhost/health"}, httpcheck.ConfigFieldURL},
			{container.Bag{httpcheck.ConfigFieldURL: "http:///health"}, httpcheck.ConfigFieldURL},
			{container.Bag{httpcheck.ConfigFieldURL: "http://local host"}, httpcheck.ConfigFieldURL},
			{container.Bag{httpcheck.ConfigFieldClient: "main", httpcheck.ConfigFieldURL: "%zz"}, httpcheck.ConfigFieldURL},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMinStatus: "200"}, httpcheck.ConfigFieldMinStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMinStatus: 99}, httpcheck.ConfigFieldMinStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMinStatus: 200.5}, httpcheck.ConfigFieldMinStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMinStatus: int64(600)}, httpcheck.ConfigFieldMinStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMinStatus: 300}, httpcheck.ConfigFieldMaxStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMaxStatus: 600}, httpcheck.ConfigFieldMaxStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldMaxStatus: 600.0}, httpcheck.ConfigFieldMaxStatus},
			{container.Bag{httpcheck.ConfigFieldURL: target, httpcheck.ConfigFieldTimeout: "-1s"}, httpcheck.ConfigFieldTimeout},
		} {
			a := httpApp(t, "http://localhost", nil)
			check, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, tc.fields))
			testutil.Error(t, err, httpcheck.ErrInvalidField,
				"unable to create http health check: invalid http health check field",
				map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldField: tc.field})
			testutil.Equal(t, "check", check, nil)
		}
	})

	t.Run("should accept the statuses as integers and integral floats", func(t *testing.T) {
		a := httpApp(t, "http://localhost", nil)
		for _, fields := range []container.Bag{
			{httpcheck.ConfigFieldMinStatus: 100, httpcheck.ConfigFieldMaxStatus: 599},
			{httpcheck.ConfigFieldMinStatus: int64(200), httpcheck.ConfigFieldMaxStatus: int64(200)},
			{httpcheck.ConfigFieldMinStatus: 200.0, httpcheck.ConfigFieldMaxStatus: 399.0},
		} {
			fields[httpcheck.ConfigFieldURL] = target
			_, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, fields))
			testutil.NoError(t, err)
		}
	})

	t.Run("should fail with a client but without client factory", func(t *testing.T) {
		a := newApp(t, nil, httpcheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldClient: "main",
		}))
		testutil.Error(t, err, httpcheck.ErrNoFactory,
			"unable to create http health check: no client factory",
			map[string]any{httpcheck.ErrorFieldID: "search"})
	})

	t.Run("should not need the client factory without client", func(t *testing.T) {
		a := newApp(t, nil, httpcheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldURL: target,
		}))
		testutil.NoError(t, err)
	})

	t.Run("should fail for an unknown client", func(t *testing.T) {
		a := httpApp(t, "http://localhost", nil)
		_, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldClient: "other",
		}))
		testutil.Error(t, err, httpcheck.ErrUnknownClient,
			"unable to create http health check: unknown client",
			map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldClient: "other"})
	})

	t.Run("should check the readiness unless configured", func(t *testing.T) {
		a := httpApp(t, "http://localhost", nil)
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("search", httpcheck.Driver, container.Bag{httpcheck.ConfigFieldURL: target}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", check.Id(), "search")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)

		check, err = c.Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldURL: target,
			health.ConfigFieldKind:   "liveness",
		}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthLiveness)
	})
}

func Test_HTTP_Check(t *testing.T) {
	t.Run("should pass on a status in range", func(t *testing.T) {
		server := httpServer(t)
		a := httpApp(t, server.URL, nil)
		c := creatorOf(t, a)
		for _, fields := range []container.Bag{
			{httpcheck.ConfigFieldURL: server.URL + "/ok"},
			{httpcheck.ConfigFieldURL: server.URL + "/down", httpcheck.ConfigFieldMinStatus: 500, httpcheck.ConfigFieldMaxStatus: 503},
			{httpcheck.ConfigFieldClient: "main", httpcheck.ConfigFieldURL: "/ok"},
			{httpcheck.ConfigFieldClient: "main"},
		} {
			check, err := c.Create(checkBag("search", httpcheck.Driver, fields))
			testutil.NoError(t, err)
			testutil.NoError(t, check.Check(context.Background()))
		}
	})

	t.Run("should fail on a status out of range", func(t *testing.T) {
		server := httpServer(t)
		a := httpApp(t, server.URL, nil)
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("search", httpcheck.Driver, container.Bag{httpcheck.ConfigFieldURL: server.URL + "/down"}))
		testutil.NoError(t, err)
		testutil.Error(t, check.Check(context.Background()), httpcheck.ErrStatus,
			"http health check failed: unhealthy http status: 503",
			map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldURL: server.URL + "/down", httpcheck.ErrorFieldStatus: 503})

		check, err = c.Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldClient: "main",
			httpcheck.ConfigFieldURL:    "/missing",
		}))
		testutil.NoError(t, err)
		testutil.Error(t, check.Check(context.Background()), httpcheck.ErrStatus,
			"http health check failed: unhealthy http status: 404",
			map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldURL: "/missing", httpcheck.ErrorFieldStatus: 404})
	})

	t.Run("should fail when the server cannot be reached", func(t *testing.T) {
		server := httpServer(t)
		a := httpApp(t, server.URL, nil)
		c := creatorOf(t, a)
		target := server.URL + "/ok"
		server.Close()

		check, err := c.Create(checkBag("search", httpcheck.Driver, container.Bag{httpcheck.ConfigFieldURL: target}))
		testutil.NoError(t, err)
		wantErr(t, check.Check(context.Background()),
			map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldURL: target},
			httpcheck.ErrRequest)

		check, err = c.Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldClient: "main",
			httpcheck.ConfigFieldURL:    "/ok",
		}))
		testutil.NoError(t, err)
		wantErr(t, check.Check(context.Background()),
			map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldClient: "main", httpcheck.ErrorFieldURL: "/ok"},
			httpcheck.ErrRequest)
	})

	t.Run("should fail once its timeout elapsed", func(t *testing.T) {
		server := httpServer(t)
		a := httpApp(t, server.URL, nil)
		check, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldURL:     server.URL + "/slow",
			httpcheck.ConfigFieldTimeout: "20ms",
		}))
		testutil.NoError(t, err)
		err = check.Check(context.Background())
		wantErr(t, err, map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldURL: server.URL + "/slow"},
			httpcheck.ErrRequest, context.DeadlineExceeded)
	})

	t.Run("should stop once its context is done", func(t *testing.T) {
		server := httpServer(t)
		a := httpApp(t, server.URL, nil)
		check, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldURL: server.URL + "/slow",
		}))
		testutil.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := check.Check(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Check() = %v, want context.Canceled", err)
		}
	})

	t.Run("should fail when the client cannot be created", func(t *testing.T) {
		a := httpApp(t, "http://localhost", map[string]any{
			client.ClientConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "unknown"},
		})
		check, err := creatorOf(t, a).Create(checkBag("search", httpcheck.Driver, container.Bag{
			httpcheck.ConfigFieldClient: "bad",
		}))
		testutil.NoError(t, err)
		wantErr(t, check.Check(context.Background()),
			map[string]any{httpcheck.ErrorFieldID: "search", httpcheck.ErrorFieldClient: "bad"},
			httpcheck.ErrClient, factory.ErrNoCreator)
	})
}
