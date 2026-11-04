package vault

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/vault"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// token is the token of the stores of the tests.
const token = "hvs.test-token"

// fakeConfig is a flam.IConfig reading Has and Get from a bag; its other
// methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

// request is a request received by the fake server.
type request struct {
	method    string
	path      string
	token     string
	namespace string
}

// response is a response of the fake server.
type response struct {
	status int
	body   string
}

// server is a fake Vault server answering the requests of the escaped
// paths of its responses, and 404 for the others, recording the requests.
type server struct {
	*httptest.Server

	mu        sync.Mutex
	responses map[string]response
	requests  []request
	delay     time.Duration
}

// newServer returns a started fake server, or a TLS one, of the responses.
func newServer(t *testing.T, tls bool, responses map[string]response) *server {
	t.Helper()
	s := &server{responses: responses}
	handler := http.HandlerFunc(s.serve)
	if tls {
		s.Server = httptest.NewTLSServer(handler)
	} else {
		s.Server = httptest.NewServer(handler)
	}
	t.Cleanup(s.Close)
	return s
}

func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, request{
		method:    r.Method,
		path:      r.URL.EscapedPath(),
		token:     r.Header.Get(vault.HeaderToken),
		namespace: r.Header.Get(vault.HeaderNamespace),
	})
	resp, ok := s.responses[r.URL.EscapedPath()]
	delay := s.delay
	s.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
	}
	if !ok {
		resp = response{status: http.StatusNotFound, body: `{"errors":[]}`}
	}
	w.WriteHeader(resp.status)
	_, _ = w.Write([]byte(resp.body))
}

// slow delays the responses of the server.
func (s *server) slow(delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delay = delay
}

// received returns a copy of the requests received.
func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// secret returns the response of a KV version 2 secret of the data.
func secret(data string) response {
	return response{status: http.StatusOK, body: `{"request_id":"1","data":{"data":` + data + `,"metadata":{"version":1}}}`}
}

// creators returns the store creators the bundle provides in the store
// creator group, reading the defaults from a configuration of the bag, and
// the time from the clock.
func creators(t *testing.T, bag container.Bag, clock flam.IClock) []flam.ISecretStoreCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	testutil.NoError(t, vault.NewBundle().Register(app))

	var result []flam.ISecretStoreCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.ISecretStoreCreator `group:"flam.secrets.stores.creator"`
	}) {
		result = args.Creators
	}))
	return result
}

// defaults returns the bag configured by the bundle.
func defaults(t *testing.T) container.Bag {
	t.Helper()
	bag := container.Bag{}
	testutil.NoError(t, vault.NewBundle().(flam.IConfigurableBundle).Config(bag))
	return bag
}

// createWith creates a store of id "vault" from the fields, with the
// configuration of the bag and the clock.
func createWith(t *testing.T, bag container.Bag, clock flam.IClock, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "vault", factory.ConfigFieldDriver: vault.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	s, err := creators(t, bag, clock)[0].Create(cfg)
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// create creates a store of id "vault" from the fields, with the bundle
// defaults and a fake clock.
func create(t *testing.T, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	return createWith(t, defaults(t), testutil.NewClock(time.Unix(0, 0)), fields)
}

// mustCreate creates a store of the server, of the token, and of the
// fields.
func mustCreate(t *testing.T, srv *server, fields container.Bag) flam.ISecretStore {
	t.Helper()
	all := container.Bag{vault.ConfigFieldAddress: srv.URL, vault.ConfigFieldToken: token}
	for key, value := range fields.All() {
		all[key] = value
	}
	s, err := create(t, all)
	testutil.NoError(t, err)
	return s
}

// checkError checks that the error wraps the sentinel, and is a
// *flam.Error of the context, holding neither the token nor a secret.
func checkError(t *testing.T, err error, sentinel error, ctx map[string]any) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T, want a *flam.Error", err)
	}
	testutil.Context(t, e, ctx)
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("error = %v, holds a secret", err)
	}
}

// keyContext returns the context of an error of the key.
func keyContext(key string) map[string]any {
	return map[string]any{vault.ErrorFieldID: "vault", vault.ErrorFieldKey: key}
}

// noToken unsets the token variable for the test.
func noToken(t *testing.T) {
	t.Helper()
	t.Setenv(vault.EnvToken, "")
	testutil.NoError(t, os.Unsetenv(vault.EnvToken))
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{vault.Driver, "flam.secrets.stores.driver.vault"},
			{vault.EnvToken, "VAULT_TOKEN"},
			{vault.HeaderToken, "X-Vault-Token"},
			{vault.HeaderNamespace, "X-Vault-Namespace"},
			{vault.ConfigFieldAddress, "address"},
			{vault.ConfigFieldMount, "mount"},
			{vault.ConfigFieldNamespace, "namespace"},
			{vault.ConfigFieldToken, "token"},
			{vault.ConfigFieldTokenFile, "token_file"},
			{vault.ConfigFieldTimeout, "timeout"},
			{vault.ConfigFieldCAFile, "ca_file"},
			{vault.ConfigFieldSkipVerify, "skip_verify"},
			{vault.ConfigFieldCacheTTL, "cache_ttl"},
			{vault.DefaultAddressConfigPath, "flam.secrets.defaults.stores.vault.address"},
			{vault.DefaultMountConfigPath, "flam.secrets.defaults.stores.vault.mount"},
			{vault.DefaultNamespaceConfigPath, "flam.secrets.defaults.stores.vault.namespace"},
			{vault.DefaultTokenConfigPath, "flam.secrets.defaults.stores.vault.token"},
			{vault.DefaultTokenFileConfigPath, "flam.secrets.defaults.stores.vault.token_file"},
			{vault.DefaultTimeoutConfigPath, "flam.secrets.defaults.stores.vault.timeout"},
			{vault.DefaultCAFileConfigPath, "flam.secrets.defaults.stores.vault.ca_file"},
			{vault.DefaultSkipVerifyConfigPath, "flam.secrets.defaults.stores.vault.skip_verify"},
			{vault.DefaultCacheTTLConfigPath, "flam.secrets.defaults.stores.vault.cache_ttl"},
			{vault.ErrorFieldID, "id"},
			{vault.ErrorFieldField, "field"},
			{vault.ErrorFieldKey, "key"},
			{vault.ErrorFieldStatus, "status"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			vault.ErrInvalidField: "invalid vault secret store field",
			vault.ErrCAFile:       "unable to load vault certificate authorities",
			vault.ErrToken:        "unable to read vault token file",
			vault.ErrRequest:      "unable to send vault request",
			vault.ErrStatus:       "unexpected vault response status",
			vault.ErrResponse:     "invalid vault response",
		} {
			testutil.Equal(t, "message", err.Error(), text)
			testutil.Equal(t, "unwrap", errors.Unwrap(err), nil)
		}
	})
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := vault.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.stores.vault")
		if b == vault.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no store", func(t *testing.T) {
		testutil.Equal(t, "secrets", defaults(t).Get("flam.secrets"), container.Bag{
			"defaults": container.Bag{"stores": container.Bag{"vault": container.Bag{
				"address": "", "mount": "secret", "namespace": "", "token": "", "token_file": "",
				"timeout": "10s", "ca_file": "", "skip_verify": false, "cache_ttl": "30s",
			}}},
		})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := vault.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the store creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, vault.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)))), 1)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := vault.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept its driver only", func(t *testing.T) {
		c := creators(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)))[0]
		for driver, want := range map[string]bool{
			vault.Driver:                     true,
			"":                               false,
			"vault":                          false,
			"flam.secrets.stores.driver.env": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, values := range map[string][]any{
			vault.ConfigFieldAddress:    {1, "", "vault:8200", "ftp://vault", "http://", "http://%zz"},
			vault.ConfigFieldMount:      {1, "", "/", "a//b", "a/../b", "."},
			vault.ConfigFieldNamespace:  {1},
			vault.ConfigFieldToken:      {1},
			vault.ConfigFieldTokenFile:  {1},
			vault.ConfigFieldCAFile:     {1},
			vault.ConfigFieldTimeout:    {"soon", "-1s", 10},
			vault.ConfigFieldCacheTTL:   {"soon", -time.Second},
			vault.ConfigFieldSkipVerify: {"yes"},
		} {
			for _, value := range values {
				_, err := create(t, container.Bag{
					vault.ConfigFieldAddress: "http://vault:8200",
					vault.ConfigFieldToken:   token,
					field:                    value,
				})
				testutil.Error(t, err, vault.ErrInvalidField,
					"unable to create vault secret store: invalid vault secret store field",
					map[string]any{vault.ErrorFieldID: "vault", vault.ErrorFieldField: field})
			}
		}
	})

	t.Run("should refuse a store without token", func(t *testing.T) {
		noToken(t)
		_, err := create(t, container.Bag{vault.ConfigFieldAddress: "http://vault:8200"})
		testutil.Error(t, err, vault.ErrInvalidField,
			"unable to create vault secret store: invalid vault secret store field",
			map[string]any{vault.ErrorFieldID: "vault", vault.ErrorFieldField: vault.ConfigFieldToken})
	})

	t.Run("should take the token of the environment", func(t *testing.T) {
		t.Setenv(vault.EnvToken, "env-token")
		srv := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
		s, err := create(t, container.Bag{vault.ConfigFieldAddress: srv.URL})
		testutil.NoError(t, err)
		_, err = s.Get(context.Background(), "db#k")
		testutil.NoError(t, err)
		testutil.Equal(t, "token", srv.received()[0].token, "env-token")
	})

	t.Run("should take the defaults stored in the configuration", func(t *testing.T) {
		noToken(t)
		srv := newServer(t, false, map[string]response{"/v1/kv/team/data/db": secret(`{"k":"v"}`)})
		bag := defaults(t)
		for path, value := range map[string]any{
			vault.DefaultAddressConfigPath:   srv.URL + "/",
			vault.DefaultMountConfigPath:     "/kv/team/",
			vault.DefaultNamespaceConfigPath: "ns1",
			vault.DefaultTokenConfigPath:     "default-token",
			vault.DefaultTimeoutConfigPath:   time.Second,
			vault.DefaultCacheTTLConfigPath:  "0s",
		} {
			testutil.NoError(t, bag.Set(path, value))
		}
		s, err := createWith(t, bag, testutil.NewClock(time.Unix(0, 0)), container.Bag{})
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", s.Id(), "vault")
		for range 2 {
			got, err := s.Get(context.Background(), "db#k")
			testutil.NoError(t, err)
			testutil.Equal(t, "Get()", got, "v")
		}
		want := request{method: http.MethodGet, path: "/v1/kv/team/data/db", token: "default-token", namespace: "ns1"}
		testutil.Equal(t, "requests", srv.received(), []request{want, want})
	})

	t.Run("should use the built-in defaults", func(t *testing.T) {
		srv := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
		s, err := createWith(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)), container.Bag{
			vault.ConfigFieldAddress: srv.URL,
			vault.ConfigFieldToken:   token,
		})
		testutil.NoError(t, err)
		for range 2 {
			_, err := s.Get(context.Background(), "db#k")
			testutil.NoError(t, err)
		}
		testutil.Equal(t, "requests", len(srv.received()), 1)
	})

	t.Run("should fail on a missing or invalid CA file", func(t *testing.T) {
		dir := t.TempDir()
		invalid := filepath.Join(dir, "invalid.pem")
		testutil.NoError(t, os.WriteFile(invalid, []byte("not a certificate"), 0o600))
		for file, text := range map[string]string{
			filepath.Join(dir, "missing.pem"): "unable to create vault secret store: unable to load vault certificate authorities: open ",
			invalid:                           "unable to create vault secret store: unable to load vault certificate authorities",
		} {
			_, err := create(t, container.Bag{
				vault.ConfigFieldAddress: "https://vault:8200",
				vault.ConfigFieldToken:   token,
				vault.ConfigFieldCAFile:  file,
			})
			checkError(t, err, vault.ErrCAFile, map[string]any{vault.ErrorFieldID: "vault"})
			if !strings.HasPrefix(err.Error(), text) {
				t.Errorf("error = %v, want %q", err, text)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

func Test_Store_Get(t *testing.T) {
	srv := newServer(t, false, map[string]response{
		"/v1/secret/data/db":            secret(`{"password":"s3cret","user":"admin","port":5432,"tags":["a"],"empty":""}`),
		"/v1/secret/data/app/my%20conf": secret(`{"k":"v"}`),
		"/v1/secret/data/deleted":       secret(`null`),
		"/v1/secret/data/denied":        {status: http.StatusForbidden, body: `{"errors":["permission denied"]}`},
		"/v1/secret/data/broken":        {status: http.StatusOK, body: `{"data":`},
		"/v1/secret/data/scalar":        {status: http.StatusOK, body: `{"data":{"data":"s3cret"}}`},
	})

	t.Run("should read the field of the secret of the key", func(t *testing.T) {
		s := mustCreate(t, srv, container.Bag{vault.ConfigFieldNamespace: "team"})
		// in order, the last key reading the secret requested last
		for _, tc := range []struct{ key, want string }{
			{"db#password", "s3cret"},
			{"db#user", "admin"},
			{"db#port", "5432"},
			{"db#tags", `["a"]`},
			{"db#empty", ""},
			{"app/my conf#k", "v"},
		} {
			got, err := s.Get(context.Background(), tc.key)
			testutil.NoError(t, err)
			testutil.Equal(t, tc.key, got, tc.want)
		}
		requests := srv.received()
		testutil.Equal(t, "last request", requests[len(requests)-1], request{
			method: http.MethodGet, path: "/v1/secret/data/app/my%20conf", token: token, namespace: "team",
		})
	})

	t.Run("should take a nil context as the background one", func(t *testing.T) {
		var ctx context.Context
		got, err := mustCreate(t, srv, container.Bag{}).Get(ctx, "db#user")
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", got, "admin")
	})

	t.Run("should refuse the keys of another form", func(t *testing.T) {
		s := mustCreate(t, srv, container.Bag{})
		for _, key := range []string{"", "db", "db#", "#password", "/db#password", "db/#password", "a/../db#password", "./db#password"} {
			_, err := s.Get(context.Background(), key)
			checkError(t, err, secrets.ErrInvalidKey, keyContext(key))
			testutil.Equal(t, key+" text", err.Error(), "unable to get vault secret: invalid secret key")
		}
	})

	t.Run("should not find a missing secret or field", func(t *testing.T) {
		s := mustCreate(t, srv, container.Bag{})
		for _, key := range []string{"missing#password", "db#missing", "deleted#password"} {
			_, err := s.Get(context.Background(), key)
			checkError(t, err, secrets.ErrNotFound, keyContext(key))
			testutil.Equal(t, key+" text", err.Error(), "unable to get vault secret: secret not found")
		}
	})

	t.Run("should fail on an unexpected status", func(t *testing.T) {
		_, err := mustCreate(t, srv, container.Bag{}).Get(context.Background(), "denied#password")
		ctx := keyContext("denied#password")
		ctx[vault.ErrorFieldStatus] = http.StatusForbidden
		checkError(t, err, vault.ErrStatus, ctx)
		testutil.Equal(t, "text", err.Error(), "unable to get vault secret: unexpected vault response status")
	})

	t.Run("should fail on an invalid response", func(t *testing.T) {
		s := mustCreate(t, srv, container.Bag{})
		for _, key := range []string{"broken#password", "scalar#password"} {
			_, err := s.Get(context.Background(), key)
			checkError(t, err, vault.ErrResponse, keyContext(key))
		}
	})

	t.Run("should fail on a server that cannot be reached", func(t *testing.T) {
		down := newServer(t, false, nil)
		s := mustCreate(t, down, container.Bag{})
		down.Close()
		_, err := s.Get(context.Background(), "db#password")
		checkError(t, err, vault.ErrRequest, keyContext("db#password"))
	})

	t.Run("should bound the requests by the timeout", func(t *testing.T) {
		slow := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
		slow.slow(time.Second)
		_, err := mustCreate(t, slow, container.Bag{vault.ConfigFieldTimeout: "10ms"}).Get(context.Background(), "db#k")
		checkError(t, err, vault.ErrRequest, keyContext("db#k"))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want a deadline exceeded", err)
		}
	})

	t.Run("should be bound by the context only without timeout", func(t *testing.T) {
		slow := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
		slow.slow(10 * time.Millisecond)
		got, err := mustCreate(t, slow, container.Bag{vault.ConfigFieldTimeout: "0s"}).Get(context.Background(), "db#k")
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", got, "v")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = mustCreate(t, slow, container.Bag{vault.ConfigFieldTimeout: "0s"}).Get(ctx, "db#k")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want a cancellation", err)
		}
	})
}

func Test_Store_GetMap(t *testing.T) {
	srv := newServer(t, false, map[string]response{
		"/v1/secret/data/db": secret(`{"password":"s3cret","port":5432}`),
	})

	t.Run("should read every field of the secret", func(t *testing.T) {
		s := mustCreate(t, srv, container.Bag{})
		got, err := s.GetMap(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{"password": "s3cret", "port": "5432"})

		got["password"] = "changed"
		again, err := s.GetMap(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "copy", again["password"], "s3cret")
	})

	t.Run("should refuse an invalid path", func(t *testing.T) {
		s := mustCreate(t, srv, container.Bag{})
		for _, path := range []string{"", "/", "db/", "../db", "a//b"} {
			_, err := s.GetMap(context.Background(), path)
			checkError(t, err, secrets.ErrInvalidKey, keyContext(path))
			testutil.Equal(t, path+" text", err.Error(), "unable to get vault secrets: invalid secret key")
		}
	})

	t.Run("should not find a missing secret", func(t *testing.T) {
		_, err := mustCreate(t, srv, container.Bag{}).GetMap(context.Background(), "missing")
		checkError(t, err, secrets.ErrNotFound, keyContext("missing"))
		testutil.Equal(t, "text", err.Error(), "unable to get vault secrets: secret not found")
	})
}

func Test_Store_Cache(t *testing.T) {
	t.Run("should keep the secrets for the ttl", func(t *testing.T) {
		srv := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
		clock := testutil.NewClock(time.Unix(0, 0))
		s, err := createWith(t, defaults(t), clock, container.Bag{
			vault.ConfigFieldAddress:  srv.URL,
			vault.ConfigFieldToken:    token,
			vault.ConfigFieldCacheTTL: time.Minute,
		})
		testutil.NoError(t, err)

		read := func(want int) {
			t.Helper()
			got, err := s.Get(context.Background(), "db#k")
			testutil.NoError(t, err)
			testutil.Equal(t, "Get()", got, "v")
			_, err = s.GetMap(context.Background(), "db")
			testutil.NoError(t, err)
			testutil.Equal(t, "requests", len(srv.received()), want)
		}
		read(1)
		clock.Advance(time.Minute - time.Nanosecond)
		read(1)
		clock.Advance(time.Nanosecond)
		read(2)

		testutil.NoError(t, s.Close())
		read(3)
	})

	t.Run("should not keep the secrets that failed", func(t *testing.T) {
		srv := newServer(t, false, nil)
		s := mustCreate(t, srv, container.Bag{})
		for range 2 {
			_, err := s.Get(context.Background(), "db#k")
			checkError(t, err, secrets.ErrNotFound, keyContext("db#k"))
		}
		testutil.Equal(t, "requests", len(srv.received()), 2)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		srv := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
		s := mustCreate(t, srv, container.Bag{})
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				got, err := s.Get(context.Background(), "db#k")
				if err != nil || got != "v" {
					t.Errorf("Get() = %q, %v", got, err)
				}
			})
		}
		wg.Wait()
	})
}

func Test_Store_TokenFile(t *testing.T) {
	srv := newServer(t, false, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
	dir := t.TempDir()
	file := filepath.Join(dir, "token")

	// create creates a store of the token file, without cache, so that
	// each read sends a request.
	create := func(t *testing.T, fields container.Bag) flam.ISecretStore {
		t.Helper()
		all := container.Bag{
			vault.ConfigFieldAddress:   srv.URL,
			vault.ConfigFieldTokenFile: file,
			vault.ConfigFieldCacheTTL:  "0s",
		}
		for key, value := range fields.All() {
			all[key] = value
		}
		noToken(t)
		s, err := create(t, all)
		testutil.NoError(t, err)
		return s
	}

	t.Run("should read the token file on each request", func(t *testing.T) {
		s := create(t, container.Bag{})
		for _, tok := range []string{"first", "second"} {
			testutil.NoError(t, os.WriteFile(file, []byte(" "+tok+"\n"), 0o600))
			_, err := s.Get(context.Background(), "db#k")
			testutil.NoError(t, err)
			requests := srv.received()
			testutil.Equal(t, "token", requests[len(requests)-1].token, tok)
		}
	})

	t.Run("should prefer the token to the token file", func(t *testing.T) {
		testutil.NoError(t, os.WriteFile(file, []byte("from-file"), 0o600))
		_, err := create(t, container.Bag{vault.ConfigFieldToken: "from-field"}).Get(context.Background(), "db#k")
		testutil.NoError(t, err)
		requests := srv.received()
		testutil.Equal(t, "token", requests[len(requests)-1].token, "from-field")
	})

	t.Run("should fail on a missing or empty token file", func(t *testing.T) {
		s := create(t, container.Bag{})
		testutil.NoError(t, os.WriteFile(file, []byte(" \n"), 0o600))
		before := len(srv.received())
		_, err := s.Get(context.Background(), "db#k")
		checkError(t, err, vault.ErrToken, keyContext("db#k"))
		testutil.Equal(t, "text", err.Error(), "unable to get vault secret: unable to read vault token file: empty token file")

		testutil.NoError(t, os.Remove(file))
		_, err = s.GetMap(context.Background(), "db")
		checkError(t, err, vault.ErrToken, keyContext("db"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("error = %v, want a missing file", err)
		}
		testutil.Equal(t, "requests", len(srv.received()), before)
	})
}

func Test_Store_TLS(t *testing.T) {
	srv := newServer(t, true, map[string]response{"/v1/secret/data/db": secret(`{"k":"v"}`)})
	ca := filepath.Join(t.TempDir(), "ca.pem")
	testutil.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: srv.Certificate().Raw,
	}), 0o600))

	t.Run("should trust the certificate authorities of the CA file", func(t *testing.T) {
		got, err := mustCreate(t, srv, container.Bag{vault.ConfigFieldCAFile: ca}).Get(context.Background(), "db#k")
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", got, "v")
	})

	t.Run("should skip the verification when asked", func(t *testing.T) {
		got, err := mustCreate(t, srv, container.Bag{vault.ConfigFieldSkipVerify: true}).Get(context.Background(), "db#k")
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", got, "v")
	})

	t.Run("should refuse an unknown certificate authority", func(t *testing.T) {
		_, err := mustCreate(t, srv, container.Bag{}).Get(context.Background(), "db#k")
		checkError(t, err, vault.ErrRequest, keyContext("db#k"))
	})
}
