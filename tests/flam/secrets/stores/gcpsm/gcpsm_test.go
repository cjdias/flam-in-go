package gcpsm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"hash/crc32"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"go.uber.org/dig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/gcpsm"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// project is the project of the stores of the tests.
const project = "flam-test"

// fakeConfig is a flam.IConfig reading Has and Get from a bag; its other
// methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

// request is an AccessSecretVersion request received by the fake server.
type request struct {
	name          string
	authorization []string
}

// server is a fake Secret Manager serving AccessSecretVersion over gRPC on
// a local port, with the payloads of its versions, by resource name.
type server struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer

	address string

	mu       sync.Mutex
	payloads map[string]*secretmanagerpb.SecretPayload
	code     codes.Code // answered instead of the payloads, unless OK
	delay    time.Duration
	requests []request
}

// newServer returns a started fake server of the values of the latest
// versions of the secrets of the project, with their checksums.
func newServer(t *testing.T, values map[string]string) *server {
	t.Helper()
	s := &server{payloads: map[string]*secretmanagerpb.SecretPayload{}}
	for id, value := range values {
		s.put(id, "latest", value)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.NoError(t, err)
	s.address = listener.Addr().String()
	grpcServer := grpc.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcServer, s)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)
	return s
}

func (s *server) AccessSecretVersion(
	ctx context.Context,
	req *secretmanagerpb.AccessSecretVersionRequest,
) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.mu.Lock()
	s.requests = append(s.requests, request{name: req.GetName(), authorization: md.Get("authorization")})
	payload, found := s.payloads[req.GetName()]
	code, delay := s.code, s.delay
	s.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	switch {
	case code != codes.OK:
		return nil, status.Error(code, "refused")
	case !found:
		return nil, status.Error(codes.NotFound, "Secret ["+req.GetName()+"] not found")
	}
	return &secretmanagerpb.AccessSecretVersionResponse{Name: req.GetName(), Payload: payload}, nil
}

// put stores the value of the version of the secret, with its checksum.
func (s *server) put(id, version, value string) {
	checksum := int64(crc32.Checksum([]byte(value), crc32.MakeTable(crc32.Castagnoli)))
	s.putPayload(id, version, &secretmanagerpb.SecretPayload{Data: []byte(value), DataCrc32C: &checksum})
}

// putPayload stores the payload of the version of the secret.
func (s *server) putPayload(id, version string, payload *secretmanagerpb.SecretPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads["projects/"+project+"/secrets/"+id+"/versions/"+version] = payload
}

// fail makes the server answer with the code, and delay its responses.
func (s *server) fail(code codes.Code, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.code, s.delay = code, delay
}

// received returns a copy of the requests received.
func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// creators returns the store creators the bundle provides in the store
// creator group, reading the defaults from a configuration of the bag, and
// the time from the clock.
func creators(t *testing.T, bag container.Bag, clock flam.IClock) []flam.ISecretStoreCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	testutil.NoError(t, gcpsm.NewBundle().Register(app))

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
	testutil.NoError(t, gcpsm.NewBundle().(flam.IConfigurableBundle).Config(bag))
	return bag
}

// createWith creates a store of id "gcpsm" from the fields, with the
// configuration of the bag and the clock.
func createWith(t *testing.T, bag container.Bag, clock flam.IClock, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "gcpsm", factory.ConfigFieldDriver: gcpsm.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	s, err := creators(t, bag, clock)[0].Create(cfg)
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// create creates a store of id "gcpsm" from the fields, with the bundle
// defaults and a fake clock, without Application Default Credentials.
func create(t *testing.T, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "missing.json"))
	return createWith(t, defaults(t), testutil.NewClock(time.Unix(0, 0)), fields)
}

// serverFields returns the fields of an insecure store of the server and
// of the project, and the fields.
func serverFields(srv *server, fields container.Bag) container.Bag {
	all := container.Bag{
		gcpsm.ConfigFieldProject:  project,
		gcpsm.ConfigFieldEndpoint: srv.address,
		gcpsm.ConfigFieldInsecure: true,
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
// *flam.Error of the context, holding no secret.
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
	for _, secret := range []string{"s3cret", "PRIVATE KEY"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error = %v, holds a secret", err)
		}
	}
}

// keyContext returns the context of an error of the key.
func keyContext(key string) map[string]any {
	return map[string]any{gcpsm.ErrorFieldID: "gcpsm", gcpsm.ErrorFieldKey: key}
}

// serviceAccount returns the JSON of the credentials of a service account
// with a new private key.
func serviceAccount(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	testutil.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	testutil.NoError(t, err)
	data, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     project,
		"private_key_id": "key-id",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "flam@" + project + ".iam.gserviceaccount.com",
		"client_id":      "1",
		"token_uri":      "https://oauth2.googleapis.com/token",
	})
	testutil.NoError(t, err)
	return string(data)
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{gcpsm.Driver, "flam.secrets.stores.driver.gcpsm"},
			{gcpsm.ConfigFieldProject, "project"},
			{gcpsm.ConfigFieldVersion, "version"},
			{gcpsm.ConfigFieldEndpoint, "endpoint"},
			{gcpsm.ConfigFieldCredentialsFile, "credentials_file"},
			{gcpsm.ConfigFieldCredentialsJSON, "credentials_json"},
			{gcpsm.ConfigFieldCredentialsType, "credentials_type"},
			{gcpsm.ConfigFieldInsecure, "insecure"},
			{gcpsm.ConfigFieldTimeout, "timeout"},
			{gcpsm.ConfigFieldCacheTTL, "cache_ttl"},
			{gcpsm.DefaultProjectConfigPath, "flam.secrets.defaults.stores.gcpsm.project"},
			{gcpsm.DefaultVersionConfigPath, "flam.secrets.defaults.stores.gcpsm.version"},
			{gcpsm.DefaultEndpointConfigPath, "flam.secrets.defaults.stores.gcpsm.endpoint"},
			{gcpsm.DefaultCredentialsFileConfigPath, "flam.secrets.defaults.stores.gcpsm.credentials_file"},
			{gcpsm.DefaultCredentialsJSONConfigPath, "flam.secrets.defaults.stores.gcpsm.credentials_json"},
			{gcpsm.DefaultCredentialsTypeConfigPath, "flam.secrets.defaults.stores.gcpsm.credentials_type"},
			{gcpsm.DefaultInsecureConfigPath, "flam.secrets.defaults.stores.gcpsm.insecure"},
			{gcpsm.DefaultTimeoutConfigPath, "flam.secrets.defaults.stores.gcpsm.timeout"},
			{gcpsm.DefaultCacheTTLConfigPath, "flam.secrets.defaults.stores.gcpsm.cache_ttl"},
			{gcpsm.ErrorFieldID, "id"},
			{gcpsm.ErrorFieldField, "field"},
			{gcpsm.ErrorFieldKey, "key"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			gcpsm.ErrInvalidField: "invalid gcpsm secret store field",
			gcpsm.ErrClient:       "unable to create gcpsm secret store client",
			gcpsm.ErrRequest:      "unable to read gcpsm secret",
			gcpsm.ErrResponse:     "invalid gcpsm response",
			gcpsm.ErrFormat:       "gcpsm secret is not a JSON object",
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
		b := gcpsm.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.stores.gcpsm")
		if b == gcpsm.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no store", func(t *testing.T) {
		testutil.Equal(t, "secrets", defaults(t).Get("flam.secrets"), container.Bag{
			"defaults": container.Bag{"stores": container.Bag{"gcpsm": container.Bag{
				"project": "", "version": "latest", "endpoint": "", "credentials_file": "",
				"credentials_json": "", "credentials_type": "service_account", "insecure": false,
				"timeout": "10s", "cache_ttl": "30s",
			}}},
		})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := gcpsm.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the store creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, gcpsm.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)))), 1)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := gcpsm.NewBundle().Register(app); !errors.Is(err, boom) {
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
			gcpsm.Driver:                       true,
			"":                                 false,
			"gcpsm":                            false,
			"flam.secrets.stores.driver.awssm": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, values := range map[string][]any{
			gcpsm.ConfigFieldProject:         {1, "", "a/b"},
			gcpsm.ConfigFieldVersion:         {1, "", "1/2"},
			gcpsm.ConfigFieldEndpoint:        {1},
			gcpsm.ConfigFieldCredentialsFile: {1},
			gcpsm.ConfigFieldCredentialsJSON: {1},
			gcpsm.ConfigFieldCredentialsType: {1, "", "api_key"},
			gcpsm.ConfigFieldInsecure:        {"yes"},
			gcpsm.ConfigFieldTimeout:         {"soon", "-1s", 10},
			gcpsm.ConfigFieldCacheTTL:        {"soon", -time.Second},
		} {
			for _, value := range values {
				_, err := create(t, container.Bag{gcpsm.ConfigFieldProject: project, field: value})
				testutil.Error(t, err, gcpsm.ErrInvalidField,
					"unable to create gcpsm secret store: invalid gcpsm secret store field",
					map[string]any{gcpsm.ErrorFieldID: "gcpsm", gcpsm.ErrorFieldField: field})
			}
		}
	})

	t.Run("should refuse both credentials, and insecure without endpoint", func(t *testing.T) {
		for field, fields := range map[string]container.Bag{
			gcpsm.ConfigFieldCredentialsJSON: {gcpsm.ConfigFieldCredentialsFile: "creds.json", gcpsm.ConfigFieldCredentialsJSON: "{}"},
			gcpsm.ConfigFieldEndpoint:        {gcpsm.ConfigFieldInsecure: true},
		} {
			fields[gcpsm.ConfigFieldProject] = project
			_, err := create(t, fields)
			checkError(t, err, gcpsm.ErrInvalidField, map[string]any{
				gcpsm.ErrorFieldID: "gcpsm", gcpsm.ErrorFieldField: field,
			})
		}
	})

	t.Run("should require a project", func(t *testing.T) {
		_, err := create(t, container.Bag{})
		checkError(t, err, gcpsm.ErrInvalidField, map[string]any{
			gcpsm.ErrorFieldID: "gcpsm", gcpsm.ErrorFieldField: gcpsm.ConfigFieldProject,
		})
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		srv := newServer(t, nil)
		srv.put("db", "3", "s3cret")
		bag := defaults(t)
		for path, value := range map[string]any{
			gcpsm.DefaultProjectConfigPath:  project,
			gcpsm.DefaultVersionConfigPath:  "3",
			gcpsm.DefaultEndpointConfigPath: srv.address,
			gcpsm.DefaultInsecureConfigPath: true,
		} {
			testutil.NoError(t, bag.Set(path, value))
		}
		s, err := createWith(t, bag, testutil.NewClock(time.Unix(0, 0)), container.Bag{})
		testutil.NoError(t, err)
		value, err := s.Get(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should take the built-in defaults without the bundle defaults", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s, err := createWith(t, container.Bag{}, testutil.NewClock(time.Unix(0, 0)), serverFields(srv, nil))
		testutil.NoError(t, err)
		_, err = s.Get(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "name", srv.received()[0].name, "projects/"+project+"/secrets/db/versions/latest")
	})

	t.Run("should load the credentials of a file, or of a JSON", func(t *testing.T) {
		credentials := serviceAccount(t)
		file := filepath.Join(t.TempDir(), "credentials.json")
		testutil.NoError(t, os.WriteFile(file, []byte(credentials), 0o600))
		for _, fields := range []container.Bag{
			{gcpsm.ConfigFieldCredentialsFile: file},
			{gcpsm.ConfigFieldCredentialsJSON: credentials},
			{gcpsm.ConfigFieldCredentialsJSON: credentials, gcpsm.ConfigFieldEndpoint: "localhost:1"},
		} {
			fields[gcpsm.ConfigFieldProject] = project
			_, err := create(t, fields)
			testutil.NoError(t, err)
		}
	})

	t.Run("should fail when the credentials cannot be loaded", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.json")
		_, err := create(t, container.Bag{gcpsm.ConfigFieldProject: project, gcpsm.ConfigFieldCredentialsFile: missing})
		checkError(t, err, gcpsm.ErrClient, map[string]any{gcpsm.ErrorFieldID: "gcpsm"})
		if !strings.Contains(err.Error(), "missing.json") {
			t.Fatalf("Error() = %q, want the error of the loading", err.Error())
		}

		_, err = create(t, container.Bag{gcpsm.ConfigFieldProject: project})
		checkError(t, err, gcpsm.ErrClient, map[string]any{gcpsm.ErrorFieldID: "gcpsm"})
	})

	t.Run("should drop the error of the credentials JSON", func(t *testing.T) {
		_, err := create(t, container.Bag{
			gcpsm.ConfigFieldProject: project, gcpsm.ConfigFieldCredentialsJSON: `{"private_key": "s3cret`,
		})
		testutil.Error(t, err, gcpsm.ErrClient,
			"unable to create gcpsm secret store: unable to create gcpsm secret store client",
			map[string]any{gcpsm.ErrorFieldID: "gcpsm"})
	})
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

func Test_Store_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("should return the whole value of a secret id", func(t *testing.T) {
		srv := newServer(t, map[string]string{"api-key": "s3cret"})
		s := mustCreate(t, srv, nil)
		testutil.Equal(t, "Id()", s.Id(), "gcpsm")
		value, err := s.Get(ctx, "api-key")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
		testutil.Equal(t, "requests", srv.received(), []request{
			{name: "projects/" + project + "/secrets/api-key/versions/latest"},
		})
	})

	t.Run("should return the field of the JSON object", func(t *testing.T) {
		srv := newServer(t, map[string]string{
			"db_credentials": `{"username": "app", "password": "s3cret", "port": 5432, "tls": true, "opts": {"a": [1, 2.5]}}`,
		})
		s := mustCreate(t, srv, nil)
		for key, want := range map[string]string{
			"db_credentials#password": "s3cret",
			"db_credentials#port":     "5432",
			"db_credentials#tls":      "true",
			"db_credentials#opts":     `{"a":[1,2.5]}`,
		} {
			value, err := s.Get(ctx, key)
			testutil.NoError(t, err)
			testutil.Equal(t, key, value, want)
		}
	})

	t.Run("should read the configured version", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "latest"})
		srv.put("db", "2", "s3cret")
		s := mustCreate(t, srv, container.Bag{gcpsm.ConfigFieldVersion: "2"})
		value, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should accept a payload without checksum", func(t *testing.T) {
		srv := newServer(t, nil)
		srv.putPayload("db", "latest", &secretmanagerpb.SecretPayload{Data: []byte("s3cret")})
		s := mustCreate(t, srv, nil)
		value, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should fail for a payload not matching its checksum", func(t *testing.T) {
		srv := newServer(t, nil)
		checksum := int64(1)
		srv.putPayload("db", "latest", &secretmanagerpb.SecretPayload{Data: []byte("s3cret"), DataCrc32C: &checksum})
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "db")
		testutil.Error(t, err, gcpsm.ErrResponse, "unable to get gcpsm secret: invalid gcpsm response", keyContext("db"))
	})

	t.Run("should refuse the invalid keys", func(t *testing.T) {
		srv := newServer(t, nil)
		s := mustCreate(t, srv, nil)
		for _, key := range []string{"", "#field", "db#", "a/b", "a.b", strings.Repeat("a", 256)} {
			_, err := s.Get(ctx, key)
			testutil.Error(t, err, secrets.ErrInvalidKey,
				"unable to get gcpsm secret: invalid secret key", keyContext(key))
		}
		testutil.Equal(t, "requests", len(srv.received()), 0)
	})

	t.Run("should fail for a missing secret or field", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": `{"user": "app"}`})
		s := mustCreate(t, srv, nil)
		for _, key := range []string{"missing", "missing#password", "db#password"} {
			_, err := s.Get(ctx, key)
			testutil.Error(t, err, secrets.ErrNotFound, "unable to get gcpsm secret: secret not found", keyContext(key))
		}
	})

	t.Run("should fail for a field of a value that is not a JSON object", func(t *testing.T) {
		srv := newServer(t, map[string]string{
			"plain": "s3cret", "array": `["s3cret"]`, "null": "null", "trailing": `{"a": "s3cret"} {}`,
		})
		s := mustCreate(t, srv, nil)
		for _, name := range []string{"plain", "array", "null", "trailing"} {
			_, err := s.Get(ctx, name+"#a")
			testutil.Error(t, err, gcpsm.ErrFormat,
				"unable to get gcpsm secret: gcpsm secret is not a JSON object", keyContext(name+"#a"))
		}
	})

	t.Run("should fail for an error of the service", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(codes.PermissionDenied, 0)
		s := mustCreate(t, srv, nil)
		_, err := s.Get(ctx, "db")
		checkError(t, err, gcpsm.ErrRequest, keyContext("db"))
		if !strings.Contains(err.Error(), "PermissionDenied") {
			t.Fatalf("Error() = %q, want the status", err.Error())
		}
	})

	t.Run("should bound the read with the timeout", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(codes.OK, time.Minute)
		s := mustCreate(t, srv, container.Bag{gcpsm.ConfigFieldTimeout: "50ms"})
		_, err := s.Get(ctx, "db")
		checkError(t, err, gcpsm.ErrRequest, keyContext("db"))
	})

	t.Run("should not bound the read with a zero timeout, and accept a nil context", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		srv.fail(codes.OK, 50*time.Millisecond)
		s := mustCreate(t, srv, container.Bag{gcpsm.ConfigFieldTimeout: "0s"})
		var nilCtx context.Context
		value, err := s.Get(nilCtx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
	})

	t.Run("should fail once closed", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": "s3cret"})
		s := mustCreate(t, srv, nil)
		testutil.NoError(t, s.Close())
		_, err := s.Get(ctx, "db")
		checkError(t, err, gcpsm.ErrRequest, keyContext("db"))
	})
}

func Test_Store_GetMap(t *testing.T) {
	ctx := context.Background()

	t.Run("should return the fields of the JSON object, in a new map", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": `{"username": "app", "password": "s3cret", "port": 5432}`})
		s := mustCreate(t, srv, nil)
		values, err := s.GetMap(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "values", values, map[string]string{"username": "app", "password": "s3cret", "port": "5432"})
		values["password"] = "changed"
		again, err := s.GetMap(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "password", again["password"], "s3cret")
		testutil.Equal(t, "requests", len(srv.received()), 1)
	})

	t.Run("should refuse the invalid ids", func(t *testing.T) {
		s := mustCreate(t, newServer(t, nil), nil)
		for _, name := range []string{"", "db#password", "a/b"} {
			_, err := s.GetMap(ctx, name)
			testutil.Error(t, err, secrets.ErrInvalidKey,
				"unable to get gcpsm secrets: invalid secret key", keyContext(name))
		}
	})

	t.Run("should fail for a missing secret, or one that is not a JSON object", func(t *testing.T) {
		srv := newServer(t, map[string]string{"plain": "s3cret"})
		s := mustCreate(t, srv, nil)
		_, err := s.GetMap(ctx, "missing")
		testutil.Error(t, err, secrets.ErrNotFound, "unable to get gcpsm secrets: secret not found", keyContext("missing"))
		_, err = s.GetMap(ctx, "plain")
		testutil.Error(t, err, gcpsm.ErrFormat,
			"unable to get gcpsm secrets: gcpsm secret is not a JSON object", keyContext("plain"))
	})
}

func Test_Store_Cache(t *testing.T) {
	ctx := context.Background()

	t.Run("should keep the values for the ttl of the clock", func(t *testing.T) {
		srv := newServer(t, map[string]string{"db": `{"password": "s3cret"}`})
		clock := testutil.NewClock(time.Unix(0, 0))
		s, err := createWith(t, defaults(t), clock, serverFields(srv, container.Bag{gcpsm.ConfigFieldCacheTTL: "1m"}))
		testutil.NoError(t, err)

		for range 3 {
			value, err := s.Get(ctx, "db#password")
			testutil.NoError(t, err)
			testutil.Equal(t, "value", value, "s3cret")
		}
		_, err = s.GetMap(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "requests", len(srv.received()), 1)

		srv.put("db", "latest", `{"password": "rotated"}`)
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
		s := mustCreate(t, srv, container.Bag{gcpsm.ConfigFieldCacheTTL: "0s"})
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
		srv.put("db", "latest", "s3cret")
		value, err := s.Get(ctx, "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "s3cret")
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
			})
		}
		wg.Wait()
	})
}
