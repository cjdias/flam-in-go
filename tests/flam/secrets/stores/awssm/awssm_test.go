package awssm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/awssm"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// Credentials of the stores of the tests.
const (
	accessKeyID     = "AKIDFLAMTEST"
	secretAccessKey = "flam-secret-access-key"
)

// fakeConfig is a flam.IConfig reading Has and Get from a bag; its other
// methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

// request is a GetSecretValue request received by the fake server.
type request struct {
	target        string
	secretID      string
	versionStage  string
	authorization string
	securityToken string
}

// secret is a secret of the fake server: its string, or binary, value.
type secret struct {
	text   *string
	binary []byte
}

// server is a fake Secrets Manager serving the GetSecretValue action of
// its JSON API, recording the requests.
type server struct {
	*httptest.Server

	mu       sync.Mutex
	secrets  map[string]secret
	status   int    // answered instead of the secrets, unless zero
	errType  string // of the error answered with the status
	delay    time.Duration
	requests []request
}

// newServer returns a started fake server of the string secrets.
func newServer(t *testing.T, values map[string]string) *server {
	t.Helper()
	s := &server{secrets: map[string]secret{}}
	for name, value := range values {
		s.secrets[name] = secret{text: &value}
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SecretID     string `json:"SecretId"`
		VersionStage string `json:"VersionStage"`
	}
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &input)

	s.mu.Lock()
	s.requests = append(s.requests, request{
		target:        r.Header.Get("X-Amz-Target"),
		secretID:      input.SecretID,
		versionStage:  input.VersionStage,
		authorization: r.Header.Get("Authorization"),
		securityToken: r.Header.Get("X-Amz-Security-Token"),
	})
	sec, found := s.secrets[input.SecretID]
	status, errType, delay := s.status, s.errType, s.delay
	s.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if status == 0 && !found {
		status, errType = http.StatusBadRequest, "ResourceNotFoundException"
	}
	if status != 0 {
		w.Header().Set("X-Amzn-ErrorType", errType)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": errType, "Message": "refused"})
		return
	}
	output := map[string]any{"ARN": "arn:aws:secretsmanager:eu-west-1:1:secret:" + input.SecretID, "Name": input.SecretID}
	if sec.text != nil {
		output["SecretString"] = *sec.text
	}
	if sec.binary != nil {
		output["SecretBinary"] = sec.binary
	}
	_ = json.NewEncoder(w).Encode(output)
}

// fail makes the server answer with the status and error type, and delay
// its responses.
func (s *server) fail(status int, errType string, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.errType, s.delay = status, errType, delay
}

// store changes a secret of the server.
func (s *server) store(name string, sec secret) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[name] = sec
}

// received returns a copy of the requests received.
func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// text returns a pointer to the string.
func text(value string) *string {
	return &value
}

// noEnv isolates the test from the AWS environment variables and shared
// files of the user, returning the directory of the shared files it
// points to, which do not exist.
func noEnv(t *testing.T) string {
	t.Helper()
	for _, name := range []string{
		"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL_SECRETS_MANAGER",
		"AWS_MAX_ATTEMPTS", "AWS_RETRY_MODE", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI", "AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN",
	} {
		t.Setenv(name, "")
		testutil.NoError(t, os.Unsetenv(name))
	}
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	return dir
}

// creators returns the store creators the bundle provides in the store
// creator group, reading the defaults from a configuration of the bag, and
// the time from the clock.
func creators(t *testing.T, bag container.Bag, clock flam.IClock) []flam.ISecretStoreCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	testutil.NoError(t, awssm.NewBundle().Register(app))

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
	testutil.NoError(t, awssm.NewBundle().(flam.IConfigurableBundle).Config(bag))
	return bag
}

// createWith creates a store of id "awssm" from the fields, with the
// configuration of the bag and the clock.
func createWith(t *testing.T, bag container.Bag, clock flam.IClock, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "awssm", factory.ConfigFieldDriver: awssm.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	s, err := creators(t, bag, clock)[0].Create(cfg)
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// create creates a store of id "awssm" from the fields, with the bundle
// defaults and a fake clock, isolated from the AWS environment.
func create(t *testing.T, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	noEnv(t)
	return createWith(t, defaults(t), testutil.NewClock(time.Unix(0, 0)), fields)
}

// serverFields returns the fields of a store of the server, with its
// region, static credentials and a single attempt, and the fields.
func serverFields(srv *server, fields container.Bag) container.Bag {
	all := container.Bag{
		awssm.ConfigFieldEndpoint:        srv.URL,
		awssm.ConfigFieldRegion:          "eu-west-1",
		awssm.ConfigFieldAccessKeyID:     accessKeyID,
		awssm.ConfigFieldSecretAccessKey: secretAccessKey,
		awssm.ConfigFieldMaxAttempts:     1,
	}
	for key, value := range fields.All() {
		all[key] = value
	}
	return all
}

// mustCreate creates a store of the server and of the fields.
func mustCreate(t *testing.T, srv *server, fields container.Bag) flam.ISecretStore {
	t.Helper()
	s, err := create(t, serverFields(srv, fields))
	testutil.NoError(t, err)
	return s
}

// checkError checks that the error wraps the sentinel, and is a
// *flam.Error of the context, holding neither the credentials nor a
// secret.
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
	for _, secret := range []string{secretAccessKey, "s3cret"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error = %v, holds a secret", err)
		}
	}
}

// keyContext returns the context of an error of the key.
func keyContext(key string) map[string]any {
	return map[string]any{awssm.ErrorFieldID: "awssm", awssm.ErrorFieldKey: key}
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{awssm.Driver, "flam.secrets.stores.driver.awssm"},
			{awssm.ConfigFieldRegion, "region"},
			{awssm.ConfigFieldEndpoint, "endpoint"},
			{awssm.ConfigFieldProfile, "profile"},
			{awssm.ConfigFieldAccessKeyID, "access_key_id"},
			{awssm.ConfigFieldSecretAccessKey, "secret_access_key"},
			{awssm.ConfigFieldSessionToken, "session_token"},
			{awssm.ConfigFieldVersionStage, "version_stage"},
			{awssm.ConfigFieldTimeout, "timeout"},
			{awssm.ConfigFieldMaxAttempts, "max_attempts"},
			{awssm.ConfigFieldCacheTTL, "cache_ttl"},
			{awssm.DefaultRegionConfigPath, "flam.secrets.defaults.stores.awssm.region"},
			{awssm.DefaultEndpointConfigPath, "flam.secrets.defaults.stores.awssm.endpoint"},
			{awssm.DefaultProfileConfigPath, "flam.secrets.defaults.stores.awssm.profile"},
			{awssm.DefaultAccessKeyIDConfigPath, "flam.secrets.defaults.stores.awssm.access_key_id"},
			{awssm.DefaultSecretAccessKeyConfigPath, "flam.secrets.defaults.stores.awssm.secret_access_key"},
			{awssm.DefaultSessionTokenConfigPath, "flam.secrets.defaults.stores.awssm.session_token"},
			{awssm.DefaultVersionStageConfigPath, "flam.secrets.defaults.stores.awssm.version_stage"},
			{awssm.DefaultTimeoutConfigPath, "flam.secrets.defaults.stores.awssm.timeout"},
			{awssm.DefaultMaxAttemptsConfigPath, "flam.secrets.defaults.stores.awssm.max_attempts"},
			{awssm.DefaultCacheTTLConfigPath, "flam.secrets.defaults.stores.awssm.cache_ttl"},
			{awssm.ErrorFieldID, "id"},
			{awssm.ErrorFieldField, "field"},
			{awssm.ErrorFieldKey, "key"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			awssm.ErrInvalidField: "invalid awssm secret store field",
			awssm.ErrClient:       "unable to create awssm secret store client",
			awssm.ErrRequest:      "unable to read awssm secret",
			awssm.ErrResponse:     "invalid awssm response",
			awssm.ErrFormat:       "awssm secret is not a JSON object",
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
		b := awssm.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.stores.awssm")
		if b == awssm.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no store", func(t *testing.T) {
		testutil.Equal(t, "secrets", defaults(t).Get("flam.secrets"), container.Bag{
			"defaults": container.Bag{"stores": container.Bag{"awssm": container.Bag{
				"region": "", "endpoint": "", "profile": "", "access_key_id": "", "secret_access_key": "",
				"session_token": "", "version_stage": "AWSCURRENT", "timeout": "10s", "max_attempts": 3,
				"cache_ttl": "30s",
			}}},
		})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := awssm.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the store creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, awssm.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)))), 1)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := awssm.NewBundle().Register(app); !errors.Is(err, boom) {
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
			awssm.Driver:                       true,
			"":                                 false,
			"awssm":                            false,
			"flam.secrets.stores.driver.vault": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, values := range map[string][]any{
			awssm.ConfigFieldRegion:          {1},
			awssm.ConfigFieldEndpoint:        {1, "localhost:4566", "ftp://aws", "http://", "http://%zz"},
			awssm.ConfigFieldProfile:         {1},
			awssm.ConfigFieldAccessKeyID:     {1},
			awssm.ConfigFieldSecretAccessKey: {1},
			awssm.ConfigFieldSessionToken:    {1},
			awssm.ConfigFieldVersionStage:    {1, ""},
			awssm.ConfigFieldTimeout:         {"soon", "-1s", 10},
			awssm.ConfigFieldCacheTTL:        {"soon", -time.Second},
			awssm.ConfigFieldMaxAttempts:     {"3", 0, -1},
		} {
			for _, value := range values {
				_, err := create(t, container.Bag{field: value})
				testutil.Error(t, err, awssm.ErrInvalidField,
					"unable to create awssm secret store: invalid awssm secret store field",
					map[string]any{awssm.ErrorFieldID: "awssm", awssm.ErrorFieldField: field})
			}
		}
	})

	t.Run("should refuse incomplete static credentials", func(t *testing.T) {
		for field, fields := range map[string]container.Bag{
			awssm.ConfigFieldAccessKeyID:     {awssm.ConfigFieldSecretAccessKey: secretAccessKey},
			awssm.ConfigFieldSecretAccessKey: {awssm.ConfigFieldAccessKeyID: accessKeyID},
		} {
			_, err := create(t, fields)
			checkError(t, err, awssm.ErrInvalidField, map[string]any{
				awssm.ErrorFieldID: "awssm", awssm.ErrorFieldField: field,
			})
		}
		_, err := create(t, container.Bag{awssm.ConfigFieldSessionToken: "token"})
		checkError(t, err, awssm.ErrInvalidField, map[string]any{
			awssm.ErrorFieldID: "awssm", awssm.ErrorFieldField: awssm.ConfigFieldSecretAccessKey,
		})
	})

	t.Run("should take the built-in defaults without the bundle defaults", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		noEnv(t)
		s, err := createWith(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)), container.Bag{
			awssm.ConfigFieldEndpoint: srv.URL, awssm.ConfigFieldRegion: "eu-west-1",
			awssm.ConfigFieldAccessKeyID: accessKeyID, awssm.ConfigFieldSecretAccessKey: secretAccessKey,
		})
		testutil.NoError(t, err)
		value, err := s.Get(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
		testutil.Equal(t, "stage", srv.received()[0].versionStage, "AWSCURRENT")
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		noEnv(t)
		bag := defaults(t)
		for path, value := range map[string]any{
			awssm.DefaultEndpointConfigPath:        srv.URL,
			awssm.DefaultRegionConfigPath:          "eu-west-1",
			awssm.DefaultAccessKeyIDConfigPath:     accessKeyID,
			awssm.DefaultSecretAccessKeyConfigPath: secretAccessKey,
			awssm.DefaultVersionStageConfigPath:    "AWSPENDING",
		} {
			testutil.NoError(t, bag.Set(path, value))
		}
		s, err := createWith(t, bag, testutil.NewClock(time.Unix(0, 0)), container.Bag{})
		testutil.NoError(t, err)
		_, err = s.Get(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "stage", srv.received()[0].versionStage, "AWSPENDING")
	})

	t.Run("should fail when the configuration of the SDK cannot be loaded", func(t *testing.T) {
		_, err := create(t, container.Bag{awssm.ConfigFieldProfile: "missing"})
		checkError(t, err, awssm.ErrClient, map[string]any{awssm.ErrorFieldID: "awssm"})
		if !strings.HasPrefix(err.Error(), "unable to create awssm secret store: unable to create awssm secret store client: ") {
			t.Fatalf("Error() = %q", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

func Test_Store_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("should return the whole value of a name", func(t *testing.T) {
		srv := newServer(t, map[string]string{"prod/api-key": "s3cret"})
		s := mustCreate(t, srv, nil)
		testutil.Equal(t, "Id()", s.Id(), "awssm")
		value, err := s.Get(ctx, "prod/api-key")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")

		r := srv.received()[0]
		testutil.Equal(t, "target", r.target, "secretsmanager.GetSecretValue")
		testutil.Equal(t, "secret id", r.secretID, "prod/api-key")
		testutil.Equal(t, "stage", r.versionStage, "AWSCURRENT")
		if !strings.Contains(r.authorization, "Credential="+accessKeyID+"/") ||
			!strings.Contains(r.authorization, "/eu-west-1/secretsmanager/aws4_request") {
			t.Fatalf("Authorization = %q", r.authorization)
		}
	})

	t.Run("should return the field of the JSON object", func(t *testing.T) {
		srv := newServer(t, map[string]string{
			"prod/db": `{"username": "app", "password": "s3cret", "port": 5432, "tls": true, "opts": {"a": [1, 2.5]}, "none": null}`,
		})
		s := mustCreate(t, srv, nil)
		for key, want := range map[string]string{
			"prod/db#password": "s3cret",
			"prod/db#port":     "5432",
			"prod/db#tls":      "true",
			"prod/db#opts":     `{"a":[1,2.5]}`,
			"prod/db#none":     "null",
		} {
			value, err := s.Get(ctx, key)
			testutil.NoError(t, err)
			testutil.Equal(t, key, value, want)
		}
	})

	t.Run("should return the value of a binary secret", func(t *testing.T) {
		srv := newServer(t, nil)
		srv.store("cert", secret{binary: []byte(`{"key": "s3cret"}`)})
		s := mustCreate(t, srv, nil)
		value, err := s.Get(ctx, "cert#key")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should read the configured version stage", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldVersionStage: "AWSPREVIOUS"})
		_, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "stage", srv.received()[0].versionStage, "AWSPREVIOUS")
	})

	t.Run("should send the session token", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldSessionToken: "session"})
		_, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "token", srv.received()[0].securityToken, "session")
	})

	t.Run("should refuse the invalid keys", func(t *testing.T) {
		srv := newServer(t, nil)
		s := mustCreate(t, srv, nil)
		for _, key := range []string{"", "#field", "db#"} {
			_, err := s.Get(ctx, key)
			testutil.Error(t, err, secrets.ErrInvalidKey,
				"unable to get awssm secret: invalid secret key", keyContext(key))
		}
		testutil.Equal(t, "requests", len(srv.received()), 0)
	})

	t.Run("should fail for a missing secret or field", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": `{"user": "app"}`})
		s := mustCreate(t, srv, nil)
		for _, key := range []string{"missing", "missing#password", "db#password"} {
			_, err := s.Get(ctx, key)
			testutil.Error(t, err, secrets.ErrNotFound, "unable to get awssm secret: secret not found", keyContext(key))
		}
	})

	t.Run("should fail for a field of a value that is not a JSON object", func(t *testing.T) {
		srv := newServer(t, map[string]string{
			"plain": "s3cret", "array": `["s3cret"]`, "null": "null", "trailing": `{"a": "s3cret"} {}`,
		})
		s := mustCreate(t, srv, nil)
		for _, name := range []string{"plain", "array", "null", "trailing"} {
			_, err := s.Get(ctx, name+"#a")
			testutil.Error(t, err, awssm.ErrFormat,
				"unable to get awssm secret: awssm secret is not a JSON object", keyContext(name+"#a"))
		}
	})

	t.Run("should fail for a version without value", func(t *testing.T) {
		srv := newServer(t, nil)
		srv.store("empty", secret{})
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "empty")
		testutil.Error(t, err, awssm.ErrResponse, "unable to get awssm secret: invalid awssm response", keyContext("empty"))
	})

	t.Run("should fail for an error of the service", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(http.StatusBadRequest, "AccessDeniedException", 0)
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "db")
		checkError(t, err, awssm.ErrRequest, keyContext("db"))
		if !strings.Contains(err.Error(), "AccessDeniedException") {
			t.Fatalf("Error() = %q, want the error of the service", err.Error())
		}
	})

	t.Run("should retry up to the maximum number of attempts", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(http.StatusInternalServerError, "InternalServiceError", 0)
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldMaxAttempts: 2})
		_, err := s.Get(ctx, "db")
		checkError(t, err, awssm.ErrRequest, keyContext("db"))
		testutil.Equal(t, "requests", len(srv.received()), 2)
	})

	t.Run("should bound the read with the timeout", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(0, "", time.Minute)
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldTimeout: "50ms"})
		_, err := s.Get(ctx, "db")
		checkError(t, err, awssm.ErrRequest, keyContext("db"))
	})

	t.Run("should not bound the read with a zero timeout, and accept a nil context", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(0, "", 50*time.Millisecond)
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldTimeout: "0s"})
		var nilCtx context.Context
		value, err := s.Get(nilCtx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should fail for a service that cannot be reached", func(t *testing.T) {
		srv := newServer(t, nil)
		srv.Close()
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "db")
		checkError(t, err, awssm.ErrRequest, keyContext("db"))
	})

	t.Run("should fail without region nor endpoint", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldRegion: "", awssm.ConfigFieldEndpoint: ""})
		_, err := s.Get(ctx, "db")
		checkError(t, err, awssm.ErrRequest, keyContext("db"))
	})
}

func Test_Store_GetMap(t *testing.T) {
	ctx := context.Background()

	t.Run("should return the fields of the JSON object, in a new map", func(t *testing.T) {
		srv := newServer(t, map[string]string{"prod/db": `{"username": "app", "password": "s3cret", "port": 5432}`})
		s := mustCreate(t, srv, nil)
		values, err := s.GetMap(ctx, "prod/db")
		testutil.NoError(t, err)
		testutil.Equal(t, "values", values, map[string]string{"username": "app", "password": "s3cret", "port": "5432"})
		values["password"] = "changed"
		again, err := s.GetMap(ctx, "prod/db")
		testutil.NoError(t, err)
		testutil.Equal(t, "password", again["password"], "s3cret")
		testutil.Equal(t, "requests", len(srv.received()), 1)
	})

	t.Run("should refuse the invalid names", func(t *testing.T) {
		s := mustCreate(t, newServer(t, nil), nil)
		for _, name := range []string{"", "db#password"} {
			_, err := s.GetMap(ctx, name)
			testutil.Error(t, err, secrets.ErrInvalidKey,
				"unable to get awssm secrets: invalid secret key", keyContext(name))
		}
	})

	t.Run("should fail for a missing secret, or one that is not a JSON object", func(t *testing.T) {
		srv := newServer(t, map[string]string{"plain": "s3cret"})
		s := mustCreate(t, srv, nil)
		_, err := s.GetMap(ctx, "missing")
		testutil.Error(t, err, secrets.ErrNotFound, "unable to get awssm secrets: secret not found", keyContext("missing"))
		_, err = s.GetMap(ctx, "plain")
		testutil.Error(t, err, awssm.ErrFormat,
			"unable to get awssm secrets: awssm secret is not a JSON object", keyContext("plain"))
	})
}

func Test_Store_Credentials(t *testing.T) {
	ctx := context.Background()

	t.Run("should take the credentials of the default chain", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		noEnv(t)
		t.Setenv("AWS_ACCESS_KEY_ID", "AKIDFROMENV")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")
		t.Setenv("AWS_REGION", "us-east-2")
		s, err := createWith(t, defaults(t), testutil.NewClock(time.Unix(0, 0)), container.Bag{
			awssm.ConfigFieldEndpoint: srv.URL, awssm.ConfigFieldMaxAttempts: 1,
		})
		testutil.NoError(t, err)
		_, err = s.Get(ctx, "db")
		testutil.NoError(t, err)
		if auth := srv.received()[0].authorization; !strings.Contains(auth, "Credential=AKIDFROMENV/") ||
			!strings.Contains(auth, "/us-east-2/") {
			t.Fatalf("Authorization = %q", auth)
		}
	})

	t.Run("should take the profile of the shared configuration", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		dir := noEnv(t)
		testutil.NoError(t, os.WriteFile(filepath.Join(dir, "config"),
			[]byte("[profile dev]\nregion = ap-south-1\n"), 0o600))
		testutil.NoError(t, os.WriteFile(filepath.Join(dir, "credentials"),
			[]byte("[dev]\naws_access_key_id = AKIDPROFILE\naws_secret_access_key = profile-secret\n"), 0o600))
		s, err := createWith(t, defaults(t), testutil.NewClock(time.Unix(0, 0)), container.Bag{
			awssm.ConfigFieldEndpoint: srv.URL, awssm.ConfigFieldProfile: "dev", awssm.ConfigFieldMaxAttempts: 1,
		})
		testutil.NoError(t, err)
		_, err = s.Get(ctx, "db")
		testutil.NoError(t, err)
		if auth := srv.received()[0].authorization; !strings.Contains(auth, "Credential=AKIDPROFILE/") ||
			!strings.Contains(auth, "/ap-south-1/") {
			t.Fatalf("Authorization = %q", auth)
		}
	})
}

func Test_Store_Cache(t *testing.T) {
	ctx := context.Background()

	t.Run("should keep the values for the ttl of the clock", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": `{"password": "s3cret"}`})
		clock := testutil.NewClock(time.Unix(0, 0))
		noEnv(t)
		s, err := createWith(t, defaults(t), clock, serverFields(srv, container.Bag{awssm.ConfigFieldCacheTTL: "1m"}))
		testutil.NoError(t, err)

		for range 3 {
			value, err := s.Get(ctx, "db#password")
			testutil.NoError(t, err)
			testutil.Equal(t, "value", value, "s3cret")
		}
		_, err = s.GetMap(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "requests", len(srv.received()), 1)

		srv.store("db", secret{text: text(`{"password": "rotated"}`)})
		clock.Advance(59 * time.Second)
		value, err := s.Get(ctx, "db#password")
		testutil.NoError(t, err)
		testutil.Equal(t, "cached", value, "s3cret")

		clock.Advance(time.Second)
		value, err = s.Get(ctx, "db#password")
		testutil.NoError(t, err)
		testutil.Equal(t, "expired", value, "rotated")
		testutil.Equal(t, "requests", len(srv.received()), 2)
	})

	t.Run("should not keep the values with a zero ttl", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s := mustCreate(t, srv, container.Bag{awssm.ConfigFieldCacheTTL: "0s"})
		for range 2 {
			_, err := s.Get(ctx, "db")
			testutil.NoError(t, err)
		}
		testutil.Equal(t, "requests", len(srv.received()), 2)
	})

	t.Run("should not keep the failures", func(t *testing.T) {
		srv := newServer(t, nil)
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "db")
		checkError(t, err, secrets.ErrNotFound, keyContext("db"))
		srv.store("db", secret{text: text("s3cret")})
		value, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should drop the cache when closed, and remain usable", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.NoError(t, s.Close())
		_, err = s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "requests", len(srv.received()), 2)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": `{"password": "s3cret"}`})
		s := mustCreate(t, srv, nil)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				value, err := s.Get(ctx, "db#password")
				testutil.NoError(t, err)
				testutil.Equal(t, "value", value, "s3cret")
				_, err = s.GetMap(ctx, "db")
				testutil.NoError(t, err)
				_ = s.Close()
			})
		}
		wg.Wait()
	})
}
