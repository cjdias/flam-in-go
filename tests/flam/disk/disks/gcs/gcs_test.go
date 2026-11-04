package gcs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"hash/crc32"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"google.golang.org/api/googleapi"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/gcs"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// bucket is the name of the bucket of the fake storage.
const bucket = "bucket"

// fakeConfig is a flam.IConfig reading Has, Get and Bag from a bag; its
// other methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

func (c fakeConfig) Bag(path string, def ...flam.IBag) flam.IBag { return c.bag.Bag(path, def...) }

// newApp returns a testutil.App providing a fake flam.IConfig holding the
// defaults of the configurable bundles merged with values, and with the
// disk and gcs bundles registered.
func newApp(t *testing.T, values container.Bag) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{disk.NewBundle(), gcs.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	return app
}

// diskFactory returns the flam.IDiskFactory of the app container.
func diskFactory(t *testing.T, app *testutil.App) flam.IDiskFactory {
	t.Helper()
	var f flam.IDiskFactory
	testutil.NoError(t, app.Invoke(func(provided flam.IDiskFactory) { f = provided }))
	return f
}

// creatorError returns the *flam.Error of the creator wrapped by the
// error of the factory, failing the test unless err wraps the sentinel.
func creatorError(t *testing.T, err, sentinel error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(err, &e) {
		t.Fatalf("Get() = %v, want %v", err, sentinel)
	}
	for !strings.HasPrefix(e.Error(), "unable to create gcs disk") && errors.As(e.Unwrap(), &e) {
	}
	return e
}

// object is an object of the fake storage.
type object struct {
	data    []byte
	updated time.Time
}

// server is a fake of the subset of the JSON API of Google Cloud Storage
// the disks use, holding the objects of the bucket, and serving OAuth2
// tokens. It fails with a 403 the requests fail tells.
type server struct {
	*httptest.Server

	mu      sync.Mutex
	objects map[string]object
	fail    func(r *http.Request) bool
	auth    []string
}

// newServer returns a running fake storage holding an empty bucket, and
// keeps the application default credentials of the machine from the test.
func newServer(t *testing.T) *server {
	t.Helper()
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "none.json"))
	t.Setenv("STORAGE_EMULATOR_HOST", "")
	s := &server{objects: map[string]object{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /storage/v1/b/{bucket}/o", s.list)
	mux.HandleFunc("GET /storage/v1/b/{bucket}/o/{object...}", s.get)
	mux.HandleFunc("DELETE /storage/v1/b/{bucket}/o/{object...}", s.delete)
	mux.HandleFunc("POST /upload/storage/v1/b/{bucket}/o", s.upload)
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"token","token_type":"Bearer","expires_in":3600}`)
	})
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		fail := s.fail
		s.auth = append(s.auth, r.Header.Get("Authorization"))
		s.mu.Unlock()
		if fail != nil && fail(r) {
			s.error(w, http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

// endpoint returns the URL of the JSON API of the storage.
func (s *server) endpoint() string {
	return s.URL + "/storage/v1/"
}

// failWhen makes the storage fail the requests fail tells.
func (s *server) failWhen(fail func(r *http.Request) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = fail
}

// failOn makes the storage fail the requests of the method, for the object
// of the key, any when empty, and with the query parameter, any when
// empty.
func (s *server) failOn(method, key, param string) {
	s.failWhen(func(r *http.Request) bool {
		return r.Method == method && (key == "" || objectName(r) == key) && (param == "" || r.URL.Query().Has(param))
	})
}

// objectName returns the name of the object of the request, empty for the
// requests of the bucket.
func objectName(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/storage/v1/b/"+bucket+"/o/")
}

// isList tells whether the request lists the objects of the bucket.
func isList(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == "/storage/v1/b/"+bucket+"/o"
}

// put stores the object in the bucket.
func (s *server) put(key, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = object{data: []byte(content), updated: time.Now()}
}

// keys returns the sorted keys of the objects of the bucket.
func (s *server) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := []string{}
	for key := range s.objects {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// content returns the content of the object of the bucket.
func (s *server) content(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.objects[key].data)
}

// authorizations returns the Authorization headers of the requests so far.
func (s *server) authorizations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.auth)
}

// error answers with the JSON error of the status.
func (s *server) error(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
		"code": status, "message": http.StatusText(status),
	}})
}

// resource returns the JSON resource of the object of the key.
func (s *server) resource(key string, o object) map[string]any {
	return map[string]any{
		"kind":       "storage#object",
		"bucket":     bucket,
		"name":       key,
		"size":       strconv.Itoa(len(o.data)),
		"updated":    o.updated.UTC().Format(time.RFC3339Nano),
		"generation": "1",
		"crc32c":     base64.StdEncoding.EncodeToString(binary.BigEndian.AppendUint32(nil, crc32.Checksum(o.data, crc32.MakeTable(crc32.Castagnoli)))),
	}
}

func (s *server) list(w http.ResponseWriter, r *http.Request) {
	prefix, delimiter := r.URL.Query().Get("prefix"), r.URL.Query().Get("delimiter")
	items, prefixes := []any{}, []string{}
	for _, key := range s.keys() {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if i := strings.Index(key[len(prefix):], delimiter); delimiter != "" && i >= 0 {
			if common := key[:len(prefix)+i+1]; !slices.Contains(prefixes, common) {
				prefixes = append(prefixes, common)
			}
			continue
		}
		s.mu.Lock()
		items = append(items, s.resource(key, s.objects[key]))
		s.mu.Unlock()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"kind": "storage#objects", "items": items, "prefixes": prefixes})
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("object")
	s.mu.Lock()
	o, ok := s.objects[key]
	s.mu.Unlock()
	switch {
	case !ok:
		s.error(w, http.StatusNotFound)
	case r.URL.Query().Get("alt") == "media":
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Goog-Generation", "1")
		_, _ = w.Write(o.data)
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.resource(key, o))
	}
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("object")
	s.mu.Lock()
	_, ok := s.objects[key]
	delete(s.objects, key)
	s.mu.Unlock()
	if !ok {
		s.error(w, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) upload(w http.ResponseWriter, r *http.Request) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	var metadata struct {
		Name string `json:"name"`
	}
	part, err := reader.NextPart()
	if err == nil {
		err = json.NewDecoder(part).Decode(&metadata)
	}
	if err == nil {
		part, err = reader.NextPart()
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(part)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	o := object{data: data, updated: time.Now()}
	s.mu.Lock()
	s.objects[metadata.Name] = o
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.resource(metadata.Name, o))
}

// fields returns the configuration of an anonymous disk of the storage,
// merged with the extra fields.
func (s *server) fields(extra container.Bag) container.Bag {
	fields := container.Bag{
		factory.ConfigFieldDriver: gcs.Driver,
		gcs.ConfigFieldBucket:     bucket,
		gcs.ConfigFieldEndpoint:   s.endpoint(),
		gcs.ConfigFieldAnonymous:  true,
	}
	for field, value := range extra {
		fields[field] = value
	}
	return fields
}

// disk returns the disk "gcs" of the storage, configured by fields merged
// with the extra fields.
func (s *server) disk(t *testing.T, extra container.Bag) flam.IDisk {
	t.Helper()
	f := diskFactory(t, newApp(t, container.Bag{disk.DiskConfigPath + ".gcs": s.fields(extra)}))
	d, err := f.Get("gcs")
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return d
}

// credentials returns the JSON of service account credentials whose
// tokens are served by the storage.
func (s *server) credentials(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	testutil.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	testutil.NoError(t, err)
	data, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "project",
		"private_key_id": "key",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "disk@project.iam.gserviceaccount.com",
		"client_id":      "1",
		"token_uri":      s.URL + "/token",
	})
	testutil.NoError(t, err)
	return string(data)
}

// denied fails the test unless err is a *fs.PathError, or a
// *os.LinkError for rename, of the operation wrapping the 403 of the
// storage.
func denied(t *testing.T, err error, op string) {
	t.Helper()
	assertOp(t, err, op)
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) || apiErr.Code != http.StatusForbidden {
		t.Fatalf("error = %v, want the 403 of the storage", err)
	}
}

// failed fails the test unless err is a *fs.PathError, or a *os.LinkError
// for rename, of the operation wrapping target.
func failed(t *testing.T, err error, op string, target error) {
	t.Helper()
	assertOp(t, err, op)
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want it to wrap %v", err, target)
	}
}

// assertOp fails the test unless err is a *fs.PathError, or a
// *os.LinkError for rename, of the operation.
func assertOp(t *testing.T, err error, op string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want a %s error", op)
	}
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	switch {
	case errors.As(err, &pathErr):
		testutil.Equal(t, "Op", pathErr.Op, op)
	case errors.As(err, &linkErr):
		testutil.Equal(t, "Op", linkErr.Op, op)
	default:
		t.Fatalf("error = %T %v, want a *fs.PathError or *os.LinkError", err, err)
	}
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", gcs.Driver, "flam.disk.disks.driver.gcs")
	testutil.Equal(t, "ConfigFieldBucket", gcs.ConfigFieldBucket, "bucket")
	testutil.Equal(t, "ConfigFieldPrefix", gcs.ConfigFieldPrefix, "prefix")
	testutil.Equal(t, "ConfigFieldEndpoint", gcs.ConfigFieldEndpoint, "endpoint")
	testutil.Equal(t, "ConfigFieldCredentialsFile", gcs.ConfigFieldCredentialsFile, "credentials_file")
	testutil.Equal(t, "ConfigFieldCredentialsJSON", gcs.ConfigFieldCredentialsJSON, "credentials_json")
	testutil.Equal(t, "ConfigFieldCredentialsType", gcs.ConfigFieldCredentialsType, "credentials_type")
	testutil.Equal(t, "ConfigFieldAnonymous", gcs.ConfigFieldAnonymous, "anonymous")
	testutil.Equal(t, "DefaultEndpointConfigPath", gcs.DefaultEndpointConfigPath, "flam.disk.defaults.disks.gcs.endpoint")
	testutil.Equal(t, "DefaultCredentialsTypeConfigPath", gcs.DefaultCredentialsTypeConfigPath, "flam.disk.defaults.disks.gcs.credentials_type")
	testutil.Equal(t, "DefaultAnonymousConfigPath", gcs.DefaultAnonymousConfigPath, "flam.disk.defaults.disks.gcs.anonymous")
	testutil.Equal(t, "ErrorFieldID", gcs.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", gcs.ErrorFieldField, "field")
	testutil.Equal(t, "ErrInvalidField", gcs.ErrInvalidField.Error(), "invalid gcs disk field")
	testutil.Equal(t, "ErrClient", gcs.ErrClient.Error(), "unable to create gcs disk client")
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", gcs.NewBundle().Id(), "flam.disk.disks.gcs")
	})

	t.Run("should configure the defaults only", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, gcs.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag.Get(""), container.Bag{"flam": container.Bag{
			"disk": container.Bag{"defaults": container.Bag{"disks": container.Bag{"gcs": container.Bag{
				"endpoint":         "",
				"credentials_type": "service_account",
				"anonymous":        false,
			}}}},
		}})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		if err := gcs.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail on a conflicting bag")
		}
	})

	t.Run("should provide its creator in the creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, gcs.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
	})

	t.Run("should return the error of the application on Register", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return cause }
		if err := gcs.NewBundle().Register(app); !errors.Is(err, cause) {
			t.Fatalf("Register() = %v, want it to wrap the application error", err)
		}
	})

	t.Run("should not be closable", func(t *testing.T) {
		if _, ok := gcs.NewBundle().(flam.IClosableBundle); ok {
			t.Error("the bundle implements IClosableBundle")
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should create its disks only", func(t *testing.T) {
		newServer(t)
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "other"},
			disk.DiskConfigPath + ".gcs": container.Bag{
				factory.ConfigFieldDriver: gcs.Driver, gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldAnonymous: true,
			},
		})
		f := diskFactory(t, app)
		d, err := f.Get("gcs")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", d.Id(), "gcs")
		testutil.Equal(t, "Name()", d.Name(), "GCSFs")
		if _, err := f.Get("other"); !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get(other) = %v, want factory.ErrNoCreator", err)
		}
		testutil.NoError(t, f.Close())
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		newServer(t)
		for name, s := range map[string]struct {
			fields   container.Bag
			defaults container.Bag
			field    string
		}{
			"missing bucket":        {container.Bag{}, nil, gcs.ConfigFieldBucket},
			"empty bucket":          {container.Bag{gcs.ConfigFieldBucket: ""}, nil, gcs.ConfigFieldBucket},
			"bucket not a string":   {container.Bag{gcs.ConfigFieldBucket: 1}, nil, gcs.ConfigFieldBucket},
			"prefix not a string":   {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldPrefix: 1}, nil, gcs.ConfigFieldPrefix},
			"endpoint not a string": {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldEndpoint: 1}, nil, gcs.ConfigFieldEndpoint},
			"endpoint default":      {container.Bag{gcs.ConfigFieldBucket: bucket}, container.Bag{gcs.DefaultEndpointConfigPath: 1}, gcs.ConfigFieldEndpoint},
			"file not a string":     {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsFile: 1}, nil, gcs.ConfigFieldCredentialsFile},
			"json not a string":     {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsJSON: 1}, nil, gcs.ConfigFieldCredentialsJSON},
			"json and file": {
				container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsFile: "f.json", gcs.ConfigFieldCredentialsJSON: "{}"},
				nil, gcs.ConfigFieldCredentialsJSON,
			},
			"type not a string":    {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsType: 1}, nil, gcs.ConfigFieldCredentialsType},
			"unknown type":         {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsType: "api_key"}, nil, gcs.ConfigFieldCredentialsType},
			"type default":         {container.Bag{gcs.ConfigFieldBucket: bucket}, container.Bag{gcs.DefaultCredentialsTypeConfigPath: ""}, gcs.ConfigFieldCredentialsType},
			"anonymous not a bool": {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldAnonymous: "true"}, nil, gcs.ConfigFieldAnonymous},
			"anonymous default":    {container.Bag{gcs.ConfigFieldBucket: bucket}, container.Bag{gcs.DefaultAnonymousConfigPath: 1}, gcs.ConfigFieldAnonymous},
			"anonymous with file":  {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsFile: "f.json", gcs.ConfigFieldAnonymous: true}, nil, gcs.ConfigFieldAnonymous},
			"anonymous with json":  {container.Bag{gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldCredentialsJSON: "{}", gcs.ConfigFieldAnonymous: true}, nil, gcs.ConfigFieldAnonymous},
		} {
			t.Run(name, func(t *testing.T) {
				s.fields[factory.ConfigFieldDriver] = gcs.Driver
				values := container.Bag{disk.DiskConfigPath + ".bad": s.fields}
				for path, value := range s.defaults {
					values[path] = value
				}
				_, err := diskFactory(t, newApp(t, values)).Get("bad")
				e := creatorError(t, err, gcs.ErrInvalidField)
				testutil.Equal(t, "Error()", e.Error(), "unable to create gcs disk: invalid gcs disk field")
				testutil.Context(t, e, map[string]any{"id": "bad", "field": s.field})
			})
		}
	})

	t.Run("should fail when the client cannot be created", func(t *testing.T) {
		s := newServer(t)
		_, err := diskFactory(t, newApp(t, container.Bag{
			disk.DiskConfigPath + ".bad": s.fields(container.Bag{
				gcs.ConfigFieldAnonymous: false, gcs.ConfigFieldCredentialsJSON: "not json",
			}),
		})).Get("bad")
		e := creatorError(t, err, gcs.ErrClient)
		if !strings.HasPrefix(e.Error(), "unable to create gcs disk: unable to create gcs disk client: ") {
			t.Errorf("Error() = %q, want the error of the client library", e.Error())
		}
		testutil.Context(t, e, map[string]any{"id": "bad"})
	})

	t.Run("should authenticate with the credentials", func(t *testing.T) {
		s := newServer(t)
		s.put("file.txt", "hello")
		credentials := s.credentials(t)
		path := filepath.Join(t.TempDir(), "credentials.json")
		testutil.NoError(t, os.WriteFile(path, []byte(credentials), 0o600))
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)

		for name, fields := range map[string]container.Bag{
			"json":    {gcs.ConfigFieldAnonymous: false, gcs.ConfigFieldCredentialsJSON: credentials},
			"file":    {gcs.ConfigFieldAnonymous: false, gcs.ConfigFieldCredentialsFile: path},
			"default": {gcs.ConfigFieldAnonymous: false},
		} {
			t.Run(name, func(t *testing.T) {
				d := s.disk(t, fields)
				content, err := afero.ReadFile(d, "file.txt")
				testutil.NoError(t, err)
				testutil.Equal(t, "content", string(content), "hello")
				auth := s.authorizations()
				if last := auth[len(auth)-1]; !strings.HasPrefix(last, "Bearer ") {
					t.Errorf("Authorization = %q, want a bearer token", last)
				}
			})
		}
	})

	t.Run("should send no credentials when anonymous", func(t *testing.T) {
		s := newServer(t)
		s.put("file.txt", "hello")
		d := s.disk(t, nil)
		_, err := afero.ReadFile(d, "file.txt")
		testutil.NoError(t, err)
		for _, auth := range s.authorizations() {
			testutil.Equal(t, "Authorization", auth, "")
		}
	})

	t.Run("should take the defaults from the configuration", func(t *testing.T) {
		s := newServer(t)
		s.put("root/file.txt", "hello")
		app := newApp(t, container.Bag{
			gcs.DefaultEndpointConfigPath:  s.endpoint(),
			gcs.DefaultAnonymousConfigPath: true,
			disk.DiskConfigPath + ".gcs": container.Bag{
				factory.ConfigFieldDriver: gcs.Driver, gcs.ConfigFieldBucket: bucket, gcs.ConfigFieldPrefix: "root",
			},
		})
		d, err := diskFactory(t, app).Get("gcs")
		testutil.NoError(t, err)
		content, err := afero.ReadFile(d, "file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")
	})
}

// ---------------------------------------------------------------------------
// disk
// ---------------------------------------------------------------------------

func Test_Disk(t *testing.T) {
	t.Run("should write, read, list, rename and remove files", func(t *testing.T) {
		s := newServer(t)
		s.put("outside.txt", "outside")
		d := s.disk(t, container.Bag{gcs.ConfigFieldPrefix: "root"})

		before := time.Now().Add(-time.Minute)
		testutil.NoError(t, afero.WriteFile(d, "/dir/file.txt", []byte("hello"), 0o644))
		testutil.NoError(t, d.MkdirAll("empty", 0o755))
		testutil.Equal(t, "keys", s.keys(), []string{"outside.txt", "root/dir/file.txt", "root/empty/"})
		testutil.Equal(t, "content", s.content("root/dir/file.txt"), "hello")

		content, err := afero.ReadFile(d, "dir/file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")
		info, err := d.Stat("dir/file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "Size()", info.Size(), int64(5))
		if info.ModTime().Before(before) {
			t.Errorf("ModTime() = %v, want a recent time", info.ModTime())
		}

		entries, err := afero.ReadDir(d, "/")
		testutil.NoError(t, err)
		testutil.Equal(t, "entries", len(entries), 2)
		testutil.Equal(t, "dir", []any{entries[0].Name(), entries[0].IsDir()}, []any{"dir", true})
		testutil.Equal(t, "empty", []any{entries[1].Name(), entries[1].IsDir()}, []any{"empty", true})
		entries, err = afero.ReadDir(d, "dir")
		testutil.NoError(t, err)
		testutil.Equal(t, "file", []any{entries[0].Name(), entries[0].Size(), entries[0].IsDir()}, []any{"file.txt", int64(5), false})

		testutil.NoError(t, d.Rename("dir", "moved"))
		testutil.Equal(t, "keys", s.keys(), []string{"outside.txt", "root/empty/", "root/moved/file.txt"})
		testutil.NoError(t, d.Remove("empty"))
		testutil.NoError(t, d.RemoveAll("missing"))
		testutil.NoError(t, d.RemoveAll("/"))
		testutil.Equal(t, "keys", s.keys(), []string{"outside.txt"})

		_, err = d.Stat("file.txt")
		failed(t, err, "stat", fs.ErrNotExist)
	})

	t.Run("should fail when the storage fails", func(t *testing.T) {
		s := newServer(t)
		s.put("file.txt", "hello")
		s.put("dir/file.txt", "hello")
		d := s.disk(t, nil)

		s.failOn(http.MethodGet, "file.txt", "")
		_, err := d.Stat("file.txt")
		denied(t, err, "stat")
		s.failOn(http.MethodGet, "file.txt", "alt")
		_, err = d.Open("file.txt")
		denied(t, err, "open")
		s.failOn(http.MethodPost, "", "")
		denied(t, d.Mkdir("new", 0o755), "mkdir")
		s.failOn(http.MethodDelete, "", "")
		denied(t, d.Remove("file.txt"), "remove")
		s.failWhen(func(r *http.Request) bool {
			return isList(r) && r.URL.Query().Get("delimiter") == ""
		})
		_, err = d.Stat("missing")
		denied(t, err, "stat")
		s.failWhen(func(r *http.Request) bool {
			return isList(r) && r.URL.Query().Get("delimiter") == "/"
		})
		_, err = d.Open("dir")
		denied(t, err, "open")
		testutil.Equal(t, "keys", s.keys(), []string{"dir/file.txt", "file.txt"})
	})

	t.Run("should fail with fs.ErrNotExist on an object missing once stated", func(t *testing.T) {
		s := newServer(t)
		s.put("file.txt", "hello")
		d := s.disk(t, nil)
		s.failWhen(func(r *http.Request) bool {
			if r.URL.Query().Get("alt") == "media" {
				s.mu.Lock()
				delete(s.objects, "file.txt")
				s.mu.Unlock()
			}
			return false
		})
		_, err := d.Open("file.txt")
		failed(t, err, "open", fs.ErrNotExist)
	})

	t.Run("should close its client", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		testutil.NoError(t, d.(io.Closer).Close())
	})
}
