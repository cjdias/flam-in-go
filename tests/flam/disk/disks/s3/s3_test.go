package s3

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/s3"
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
// disk and s3 bundles registered.
func newApp(t *testing.T, values container.Bag) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{disk.NewBundle(), s3.NewBundle()}
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

// isolate keeps the AWS environment and shared files of the machine from
// the tests.
func isolate(t *testing.T) {
	t.Helper()
	none := filepath.Join(t.TempDir(), "none")
	for name, value := range map[string]string{
		"AWS_CONFIG_FILE":             none,
		"AWS_SHARED_CREDENTIALS_FILE": none,
		"AWS_PROFILE":                 "",
		"AWS_ACCESS_KEY_ID":           "",
		"AWS_SECRET_ACCESS_KEY":       "",
		"AWS_SESSION_TOKEN":           "",
		"AWS_REGION":                  "",
		"AWS_DEFAULT_REGION":          "",
		"AWS_ENDPOINT_URL":            "",
		"AWS_ENDPOINT_URL_S3":         "",
		"AWS_EC2_METADATA_DISABLED":   "true",
	} {
		t.Setenv(name, value)
	}
}

// server is a fake S3 storage holding the bucket, failing with a 403
// AccessDenied the requests fail tells.
type server struct {
	*httptest.Server

	backend *s3mem.Backend

	mu   sync.Mutex
	fail func(r *http.Request) bool
}

// newServer returns a running fake S3 storage holding an empty bucket.
func newServer(t *testing.T) *server {
	t.Helper()
	isolate(t)
	s := &server{backend: s3mem.New()}
	testutil.NoError(t, s.backend.CreateBucket(bucket))
	faker := gofakes3.New(s.backend).Server()
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		fail := s.fail
		s.mu.Unlock()
		if fail != nil && fail(r) {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
			return
		}
		faker.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

// failOn makes the storage fail the requests of the method whose path,
// without the bucket, is key, any key when empty, and whose query holds
// the parameter, any when empty.
func (s *server) failOn(method, key, param string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = func(r *http.Request) bool {
		return r.Method == method &&
			(key == "" || strings.TrimPrefix(r.URL.Path, "/"+bucket+"/") == key) &&
			(param == "" || r.URL.Query().Has(param))
	}
}

// failWhen makes the storage fail the requests fail tells.
func (s *server) failWhen(fail func(r *http.Request) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = fail
}

// recursiveList tells whether the request lists the objects of a prefix
// recursively, without limit.
func recursiveList(r *http.Request) bool {
	query := r.URL.Query()
	return r.Method == http.MethodGet && query.Has("list-type") && !query.Has("max-keys") && !query.Has("delimiter")
}

// reset makes the storage serve every request.
func (s *server) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = nil
}

// put stores the object in the bucket.
func (s *server) put(t *testing.T, key, content string) {
	t.Helper()
	_, err := s.backend.PutObject(bucket, key, map[string]string{}, strings.NewReader(content), int64(len(content)), nil)
	testutil.NoError(t, err)
}

// keys returns the keys of the objects of the bucket.
func (s *server) keys(t *testing.T) []string {
	t.Helper()
	list, err := s.backend.ListBucket(bucket, &gofakes3.Prefix{}, gofakes3.ListBucketPage{})
	testutil.NoError(t, err)
	keys := []string{}
	for _, item := range list.Contents {
		keys = append(keys, item.Key)
	}
	return keys
}

// content returns the content of the object of the bucket.
func (s *server) content(t *testing.T, key string) string {
	t.Helper()
	object, err := s.backend.GetObject(bucket, key, nil)
	testutil.NoError(t, err)
	defer func() { _ = object.Contents.Close() }()
	data, err := io.ReadAll(object.Contents)
	testutil.NoError(t, err)
	return string(data)
}

// fields returns the configuration of a disk of the storage, with static
// credentials, merged with the extra fields.
func (s *server) fields(extra container.Bag) container.Bag {
	fields := container.Bag{
		factory.ConfigFieldDriver: s3.Driver,
		s3.ConfigFieldBucket:      bucket,
		s3.ConfigFieldEndpoint:    s.URL,
		s3.ConfigFieldPathStyle:   true,
		s3.ConfigFieldAccessKey:   "key",
		s3.ConfigFieldSecretKey:   "secret",
	}
	for field, value := range extra {
		fields[field] = value
	}
	return fields
}

// disk returns the disk "s3" of the storage, configured by fields merged
// with the extra fields.
func (s *server) disk(t *testing.T, extra container.Bag) flam.IDisk {
	t.Helper()
	app := newApp(t, container.Bag{disk.DiskConfigPath + ".s3": s.fields(extra)})
	d, err := diskFactory(t, app).Get("s3")
	testutil.NoError(t, err)
	return d
}

// creatorError returns the *flam.Error of the creator wrapped by the
// error of the factory, failing the test unless err wraps the sentinel.
func creatorError(t *testing.T, err, sentinel error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(err, &e) {
		t.Fatalf("Get() = %v, want %v", err, sentinel)
	}
	for !strings.HasPrefix(e.Error(), "unable to create s3 disk") && errors.As(e.Unwrap(), &e) {
	}
	return e
}

// denied fails the test unless err is a *fs.PathError, or a
// *os.LinkError for rename, of the operation wrapping the 403 of the
// storage.
func denied(t *testing.T, err error, op string) {
	t.Helper()
	assertOp(t, err, op)
	var response *awshttp.ResponseError
	if !errors.As(err, &response) || response.HTTPStatusCode() != http.StatusForbidden {
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
	testutil.Equal(t, "Driver", s3.Driver, "flam.disk.disks.driver.s3")
	testutil.Equal(t, "ConfigFieldBucket", s3.ConfigFieldBucket, "bucket")
	testutil.Equal(t, "ConfigFieldPrefix", s3.ConfigFieldPrefix, "prefix")
	testutil.Equal(t, "ConfigFieldRegion", s3.ConfigFieldRegion, "region")
	testutil.Equal(t, "ConfigFieldEndpoint", s3.ConfigFieldEndpoint, "endpoint")
	testutil.Equal(t, "ConfigFieldPathStyle", s3.ConfigFieldPathStyle, "path_style")
	testutil.Equal(t, "ConfigFieldAccessKey", s3.ConfigFieldAccessKey, "access_key")
	testutil.Equal(t, "ConfigFieldSecretKey", s3.ConfigFieldSecretKey, "secret_key")
	testutil.Equal(t, "ConfigFieldSessionToken", s3.ConfigFieldSessionToken, "session_token")
	testutil.Equal(t, "DefaultRegionConfigPath", s3.DefaultRegionConfigPath, "flam.disk.defaults.disks.s3.region")
	testutil.Equal(t, "DefaultEndpointConfigPath", s3.DefaultEndpointConfigPath, "flam.disk.defaults.disks.s3.endpoint")
	testutil.Equal(t, "DefaultPathStyleConfigPath", s3.DefaultPathStyleConfigPath, "flam.disk.defaults.disks.s3.path_style")
	testutil.Equal(t, "ErrorFieldID", s3.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", s3.ErrorFieldField, "field")
	testutil.Equal(t, "ErrInvalidField", s3.ErrInvalidField.Error(), "invalid s3 disk field")
	testutil.Equal(t, "ErrConfig", s3.ErrConfig.Error(), "unable to load s3 disk configuration")
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", s3.NewBundle().Id(), "flam.disk.disks.s3")
	})

	t.Run("should configure the defaults only", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, s3.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag.Get(""), container.Bag{"flam": container.Bag{
			"disk": container.Bag{"defaults": container.Bag{"disks": container.Bag{"s3": container.Bag{
				"region":     "us-east-1",
				"endpoint":   "",
				"path_style": false,
			}}}},
		}})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		if err := s3.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail on a conflicting bag")
		}
	})

	t.Run("should provide its creator in the creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, s3.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
	})

	t.Run("should return the error of the application on Register", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return cause }
		if err := s3.NewBundle().Register(app); !errors.Is(err, cause) {
			t.Fatalf("Register() = %v, want it to wrap the application error", err)
		}
	})

	t.Run("should not be closable", func(t *testing.T) {
		if _, ok := s3.NewBundle().(flam.IClosableBundle); ok {
			t.Error("the bundle implements IClosableBundle")
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should create its disks only", func(t *testing.T) {
		isolate(t)
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "other"},
			disk.DiskConfigPath + ".s3":    container.Bag{factory.ConfigFieldDriver: s3.Driver, s3.ConfigFieldBucket: bucket},
		})
		f := diskFactory(t, app)
		d, err := f.Get("s3")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", d.Id(), "s3")
		testutil.Equal(t, "Name()", d.Name(), "S3Fs")
		if _, err := f.Get("other"); !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get(other) = %v, want factory.ErrNoCreator", err)
		}
		testutil.NoError(t, f.Close())
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		isolate(t)
		for name, s := range map[string]struct {
			fields   container.Bag
			defaults container.Bag
			field    string
		}{
			"missing bucket":         {container.Bag{}, nil, s3.ConfigFieldBucket},
			"empty bucket":           {container.Bag{s3.ConfigFieldBucket: ""}, nil, s3.ConfigFieldBucket},
			"bucket not a string":    {container.Bag{s3.ConfigFieldBucket: 1}, nil, s3.ConfigFieldBucket},
			"prefix not a string":    {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldPrefix: 1}, nil, s3.ConfigFieldPrefix},
			"region not a string":    {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldRegion: 1}, nil, s3.ConfigFieldRegion},
			"region default":         {container.Bag{s3.ConfigFieldBucket: bucket}, container.Bag{s3.DefaultRegionConfigPath: 1}, s3.ConfigFieldRegion},
			"endpoint not a string":  {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldEndpoint: 1}, nil, s3.ConfigFieldEndpoint},
			"endpoint default":       {container.Bag{s3.ConfigFieldBucket: bucket}, container.Bag{s3.DefaultEndpointConfigPath: true}, s3.ConfigFieldEndpoint},
			"path style not a bool":  {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldPathStyle: "true"}, nil, s3.ConfigFieldPathStyle},
			"path style default":     {container.Bag{s3.ConfigFieldBucket: bucket}, container.Bag{s3.DefaultPathStyleConfigPath: 1}, s3.ConfigFieldPathStyle},
			"access key not string":  {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldAccessKey: 1}, nil, s3.ConfigFieldAccessKey},
			"secret key not string":  {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldSecretKey: 1}, nil, s3.ConfigFieldSecretKey},
			"secret key without key": {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldSecretKey: "secret"}, nil, s3.ConfigFieldSecretKey},
			"key without secret key": {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldAccessKey: "key"}, nil, s3.ConfigFieldSecretKey},
			"token not a string": {
				container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldAccessKey: "key", s3.ConfigFieldSecretKey: "secret", s3.ConfigFieldSessionToken: 1},
				nil, s3.ConfigFieldSessionToken,
			},
			"token without key": {container.Bag{s3.ConfigFieldBucket: bucket, s3.ConfigFieldSessionToken: "token"}, nil, s3.ConfigFieldSessionToken},
		} {
			t.Run(name, func(t *testing.T) {
				s.fields[factory.ConfigFieldDriver] = s3.Driver
				values := container.Bag{disk.DiskConfigPath + ".bad": s.fields}
				for path, value := range s.defaults {
					values[path] = value
				}
				_, err := diskFactory(t, newApp(t, values)).Get("bad")
				e := creatorError(t, err, s3.ErrInvalidField)
				testutil.Equal(t, "Error()", e.Error(), "unable to create s3 disk: invalid s3 disk field")
				testutil.Context(t, e, map[string]any{"id": "bad", "field": s.field})
			})
		}
	})

	t.Run("should fail when the AWS configuration cannot be loaded", func(t *testing.T) {
		isolate(t)
		t.Setenv("AWS_PROFILE", "missing")
		_, err := diskFactory(t, newApp(t, container.Bag{
			disk.DiskConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: s3.Driver, s3.ConfigFieldBucket: bucket},
		})).Get("bad")
		e := creatorError(t, err, s3.ErrConfig)
		if !strings.HasPrefix(e.Error(), "unable to create s3 disk: unable to load s3 disk configuration: ") ||
			!strings.Contains(e.Error(), "missing") {
			t.Errorf("Error() = %q, want the error of the SDK", e.Error())
		}
		testutil.Context(t, e, map[string]any{"id": "bad"})
	})

	t.Run("should take the defaults from the configuration", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		app := newApp(t, container.Bag{
			s3.DefaultEndpointConfigPath:  s.URL,
			s3.DefaultPathStyleConfigPath: true,
			s3.DefaultRegionConfigPath:    "eu-west-1",
			disk.DiskConfigPath + ".s3": container.Bag{
				factory.ConfigFieldDriver: s3.Driver, s3.ConfigFieldBucket: bucket,
				s3.ConfigFieldAccessKey: "key", s3.ConfigFieldSecretKey: "secret", s3.ConfigFieldSessionToken: "token",
			},
		})
		d, err := diskFactory(t, app).Get("s3")
		testutil.NoError(t, err)
		content, err := afero.ReadFile(d, "file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")
	})

	t.Run("should take the region and credentials from the AWS environment", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		t.Setenv("AWS_REGION", "eu-west-1")
		t.Setenv("AWS_ACCESS_KEY_ID", "key")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
		d := s.disk(t, container.Bag{
			s3.ConfigFieldRegion: "", s3.ConfigFieldAccessKey: "", s3.ConfigFieldSecretKey: "",
		})
		content, err := afero.ReadFile(d, "file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")
	})

}

// ---------------------------------------------------------------------------
// disk
// ---------------------------------------------------------------------------

func Test_Disk_Files(t *testing.T) {
	t.Run("should write, read and stat files", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)

		before := time.Now().Add(-time.Minute)
		testutil.NoError(t, afero.WriteFile(d, "/dir/file.txt", []byte("hello"), 0o600))
		testutil.Equal(t, "keys", s.keys(t), []string{"dir/file.txt"})
		testutil.Equal(t, "content", s.content(t, "dir/file.txt"), "hello")

		content, err := afero.ReadFile(d, "dir/../dir/file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")

		info, err := d.Stat("dir/file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "Name()", info.Name(), "file.txt")
		testutil.Equal(t, "Size()", info.Size(), int64(5))
		testutil.Equal(t, "Mode()", info.Mode(), fs.FileMode(0o644))
		testutil.Equal(t, "IsDir()", info.IsDir(), false)
		testutil.Equal(t, "Sys()", info.Sys(), nil)
		if info.ModTime().Before(before) {
			t.Errorf("ModTime() = %v, want a recent time", info.ModTime())
		}
	})

	t.Run("should keep the paths inside the prefix", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "outside.txt", "outside")
		d := s.disk(t, container.Bag{s3.ConfigFieldPrefix: "/root/"})

		testutil.NoError(t, afero.WriteFile(d, "../../file.txt", []byte("hello"), 0o644))
		testutil.Equal(t, "keys", s.keys(t), []string{"outside.txt", "root/file.txt"})
		if _, err := d.Stat("../outside.txt"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Stat() outside the prefix = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("should fail with fs.ErrNotExist on a missing file", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		_, err := d.Stat("missing.txt")
		failed(t, err, "stat", fs.ErrNotExist)
		_, err = d.Open("missing.txt")
		failed(t, err, "open", fs.ErrNotExist)
	})

	t.Run("should honour the flags of OpenFile", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		d := s.disk(t, nil)

		_, err := d.OpenFile("file.txt", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		failed(t, err, "open", fs.ErrExist)

		f, err := d.OpenFile("file.txt", os.O_WRONLY|os.O_APPEND, 0o644)
		testutil.NoError(t, err)
		_, err = f.WriteString(" world")
		testutil.NoError(t, err)
		testutil.Equal(t, "content before Sync", s.content(t, "file.txt"), "hello")
		testutil.NoError(t, f.Sync())
		testutil.Equal(t, "content after Sync", s.content(t, "file.txt"), "hello world")
		testutil.NoError(t, f.Close())

		f, err = d.OpenFile("file.txt", os.O_RDWR, 0o644)
		testutil.NoError(t, err)
		_, err = f.WriteString("HELLO")
		testutil.NoError(t, err)
		testutil.NoError(t, f.Close())
		testutil.Equal(t, "content", s.content(t, "file.txt"), "HELLO world")

		f, err = d.OpenFile("file.txt", os.O_WRONLY|os.O_TRUNC, 0o644)
		testutil.NoError(t, err)
		info, err := f.Stat()
		testutil.NoError(t, err)
		testutil.Equal(t, "Size()", info.Size(), int64(0))
		testutil.NoError(t, f.Close())
		testutil.Equal(t, "content", s.content(t, "file.txt"), "")

		f, err = d.OpenFile("new.txt", os.O_RDONLY|os.O_CREATE, 0o644)
		testutil.NoError(t, err)
		testutil.NoError(t, f.Close())
		testutil.Equal(t, "keys", s.keys(t), []string{"file.txt", "new.txt"})
	})

	t.Run("should read and seek a file opened for reading, without writing it", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello world")
		d := s.disk(t, nil)

		f, err := d.Open("file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "Name()", f.Name(), "file.txt")
		buf := make([]byte, 5)
		_, err = f.ReadAt(buf, 6)
		testutil.NoError(t, err)
		testutil.Equal(t, "ReadAt()", string(buf), "world")
		_, err = f.Seek(6, io.SeekStart)
		testutil.NoError(t, err)
		rest, err := io.ReadAll(f)
		testutil.NoError(t, err)
		testutil.Equal(t, "rest", string(rest), "world")

		if _, err := f.Write([]byte("x")); err == nil {
			t.Error("Write() did not fail on a file opened for reading")
		}
		if _, err := f.Readdir(0); err == nil {
			t.Error("Readdir() did not fail on a file")
		}
		testutil.NoError(t, f.Sync())

		s.failOn(http.MethodPut, "", "")
		testutil.NoError(t, f.Close())
	})

	t.Run("should fail Sync and Close once closed", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		f, err := d.Create("file.txt")
		testutil.NoError(t, err)
		testutil.NoError(t, f.Close())
		failed(t, f.Close(), "close", fs.ErrClosed)
		failed(t, f.Sync(), "sync", fs.ErrClosed)
	})

	t.Run("should fail Sync and Close when the object cannot be written", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		f, err := d.Create("file.txt")
		testutil.NoError(t, err)

		s.failOn(http.MethodPut, "", "")
		denied(t, f.Sync(), "sync")
		denied(t, f.Close(), "close")
		failed(t, f.Close(), "close", fs.ErrClosed)
		testutil.Equal(t, "keys", s.keys(t), []string{})
	})

	t.Run("should fail to open when the storage fails", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		d := s.disk(t, nil)

		s.failOn(http.MethodHead, "", "")
		_, err := d.Open("file.txt")
		denied(t, err, "open")
		_, err = d.Stat("file.txt")
		denied(t, err, "stat")

		s.failOn(http.MethodGet, "file.txt", "")
		_, err = d.Open("file.txt")
		denied(t, err, "open")

		s.failOn(http.MethodGet, "", "max-keys")
		_, err = d.Open("missing.txt")
		denied(t, err, "open")
	})

	t.Run("should fail to read an object missing once stated", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		d := s.disk(t, nil)

		s.failWhen(func(r *http.Request) bool {
			if r.Method == http.MethodGet {
				_, _ = s.backend.DeleteMulti(bucket, "file.txt")
			}
			return false
		})
		_, err := d.Open("file.txt")
		failed(t, err, "open", fs.ErrNotExist)
	})

	t.Run("should not support modes, owners and times", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		failed(t, d.Chmod("file.txt", 0o600), "chmod", errors.ErrUnsupported)
		failed(t, d.Chown("file.txt", 1, 1), "chown", errors.ErrUnsupported)
		failed(t, d.Chtimes("file.txt", time.Now(), time.Now()), "chtimes", errors.ErrUnsupported)
	})

	t.Run("should do nothing on Close", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		closer, ok := d.(io.Closer)
		if !ok {
			t.Fatal("the disk is not an io.Closer")
		}
		testutil.NoError(t, closer.Close())
	})
}

func Test_Disk_Directories(t *testing.T) {
	t.Run("should stat the root and the directories of the keys", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "a/b/file.txt", "hello")
		d := s.disk(t, nil)

		for path, name := range map[string]string{"/": "/", "": "/", "a": "a", "a/b/": "b"} {
			info, err := d.Stat(path)
			testutil.NoError(t, err)
			testutil.Equal(t, "Name() of "+path, info.Name(), name)
			testutil.Equal(t, "IsDir() of "+path, info.IsDir(), true)
			testutil.Equal(t, "Mode() of "+path, info.Mode(), fs.ModeDir|0o755)
			testutil.Equal(t, "Size() of "+path, info.Size(), int64(0))
			testutil.Equal(t, "ModTime() of "+path, info.ModTime(), time.Time{})
		}
	})

	t.Run("should make directories with markers", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		d := s.disk(t, container.Bag{s3.ConfigFieldPrefix: "root"})

		testutil.NoError(t, d.Mkdir("a", 0o755))
		testutil.NoError(t, d.MkdirAll("b/c", 0o755))
		testutil.NoError(t, d.MkdirAll("b", 0o755))
		testutil.NoError(t, d.MkdirAll("/", 0o755))
		testutil.Equal(t, "keys", s.keys(t), []string{"file.txt", "root/a/", "root/b/c/"})

		failed(t, d.Mkdir("a", 0o755), "mkdir", fs.ErrExist)
		failed(t, d.Mkdir("/", 0o755), "mkdir", fs.ErrExist)
		testutil.NoError(t, afero.WriteFile(d, "file.txt", nil, 0o644))
		failed(t, d.MkdirAll("file.txt", 0o755), "mkdir", syscall.ENOTDIR)
	})

	t.Run("should fail to make directories when the storage fails", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)

		s.failOn(http.MethodHead, "", "")
		denied(t, d.Mkdir("a", 0o755), "mkdir")
		denied(t, d.MkdirAll("a", 0o755), "mkdir")

		s.failOn(http.MethodPut, "", "")
		denied(t, d.Mkdir("a", 0o755), "mkdir")
		denied(t, d.MkdirAll("a", 0o755), "mkdir")
	})

	t.Run("should read the entries of a directory", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "dir/", "")
		s.put(t, "dir/b.txt", "bb")
		s.put(t, "dir/a.txt", "a")
		s.put(t, "dir/sub/c.txt", "c")
		d := s.disk(t, nil)

		f, err := d.Open("dir")
		testutil.NoError(t, err)
		testutil.Equal(t, "Name()", f.Name(), "dir")
		info, err := f.Stat()
		testutil.NoError(t, err)
		testutil.Equal(t, "IsDir()", info.IsDir(), true)

		entries, err := f.Readdir(2)
		testutil.NoError(t, err)
		testutil.Equal(t, "entries", len(entries), 2)
		testutil.Equal(t, "first", []any{entries[0].Name(), entries[0].Size(), entries[0].IsDir()}, []any{"a.txt", int64(1), false})
		testutil.Equal(t, "second", []any{entries[1].Name(), entries[1].Size(), entries[1].IsDir()}, []any{"b.txt", int64(2), false})
		names, err := f.Readdirnames(5)
		testutil.NoError(t, err)
		testutil.Equal(t, "names", names, []string{"sub"})
		names, err = f.Readdirnames(1)
		testutil.Equal(t, "names at the end", names, []string{})
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Readdirnames() at the end = %v, want io.EOF", err)
		}
		entries, err = f.Readdir(0)
		testutil.NoError(t, err)
		testutil.Equal(t, "entries at the end", len(entries), 0)
		testutil.NoError(t, f.Sync())
		testutil.NoError(t, f.Close())
		failed(t, f.Close(), "close", fs.ErrClosed)
		_, err = f.Readdir(0)
		failed(t, err, "readdir", fs.ErrClosed)

		root, err := afero.ReadDir(d, "/")
		testutil.NoError(t, err)
		testutil.Equal(t, "root entries", len(root), 1)
		testutil.Equal(t, "root entry", root[0].Name(), "dir")
	})

	t.Run("should fail the content methods of a directory", func(t *testing.T) {
		s := newServer(t)
		d := s.disk(t, nil)
		f, err := d.Open("/")
		testutil.NoError(t, err)

		_, err = f.Read(nil)
		failed(t, err, "read", syscall.EISDIR)
		_, err = f.ReadAt(nil, 0)
		failed(t, err, "read", syscall.EISDIR)
		_, err = f.Seek(0, io.SeekStart)
		failed(t, err, "seek", syscall.EISDIR)
		_, err = f.Write(nil)
		failed(t, err, "write", syscall.EISDIR)
		_, err = f.WriteAt(nil, 0)
		failed(t, err, "write", syscall.EISDIR)
		_, err = f.WriteString("")
		failed(t, err, "write", syscall.EISDIR)
		failed(t, f.Truncate(0), "truncate", syscall.EISDIR)
	})

	t.Run("should not open a directory for writing", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "dir/file.txt", "hello")
		d := s.disk(t, nil)
		_, err := d.Create("dir")
		failed(t, err, "open", syscall.EISDIR)
		_, err = d.OpenFile("/", os.O_WRONLY, 0)
		failed(t, err, "open", syscall.EISDIR)
	})

	t.Run("should fail to open a directory when it cannot be listed", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "dir/file.txt", "hello")
		d := s.disk(t, nil)
		s.failOn(http.MethodGet, "", "delimiter")
		_, err := d.Open("dir")
		denied(t, err, "open")
	})
}

func Test_Disk_Remove(t *testing.T) {
	t.Run("should remove files and empty directories", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		s.put(t, "empty/", "")
		s.put(t, "full/file.txt", "hello")
		d := s.disk(t, nil)

		testutil.NoError(t, d.Remove("file.txt"))
		testutil.NoError(t, d.Remove("empty"))
		failed(t, d.Remove("full"), "remove", syscall.ENOTEMPTY)
		failed(t, d.Remove("missing"), "remove", fs.ErrNotExist)
		failed(t, d.Remove("/"), "remove", syscall.ENOTEMPTY)
		testutil.Equal(t, "keys", s.keys(t), []string{"full/file.txt"})

		testutil.NoError(t, d.Remove("full/file.txt"))
		testutil.NoError(t, d.Remove("/"))
		testutil.Equal(t, "keys", s.keys(t), []string{})
	})

	t.Run("should fail to remove when the storage fails", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		s.put(t, "empty/", "")
		d := s.disk(t, nil)

		s.failOn(http.MethodHead, "", "")
		denied(t, d.Remove("file.txt"), "remove")
		s.failOn(http.MethodDelete, "", "")
		denied(t, d.Remove("file.txt"), "remove")
		denied(t, d.Remove("empty"), "remove")
		s.failOn(http.MethodGet, "", "delimiter")
		denied(t, d.Remove("empty"), "remove")
		s.reset()
		testutil.Equal(t, "keys", s.keys(t), []string{"empty/", "file.txt"})
	})

	t.Run("should remove all under a path", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "dir", "file of the directory name")
		s.put(t, "dir/", "")
		s.put(t, "dir/a/b.txt", "b")
		s.put(t, "dir.txt", "kept")
		d := s.disk(t, nil)

		testutil.NoError(t, d.RemoveAll("dir"))
		testutil.NoError(t, d.RemoveAll("missing"))
		testutil.Equal(t, "keys", s.keys(t), []string{"dir.txt"})
		testutil.NoError(t, d.RemoveAll("/"))
		testutil.Equal(t, "keys", s.keys(t), []string{})
	})

	t.Run("should fail to remove all when the storage fails", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "dir/a.txt", "a")
		d := s.disk(t, nil)

		s.failOn(http.MethodDelete, "dir", "")
		denied(t, d.RemoveAll("dir"), "removeall")
		s.failWhen(recursiveList)
		denied(t, d.RemoveAll("dir"), "removeall")
		s.failOn(http.MethodDelete, "dir/a.txt", "")
		denied(t, d.RemoveAll("dir"), "removeall")
	})
}

func Test_Disk_Rename(t *testing.T) {
	t.Run("should rename files and directories", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		s.put(t, "dir/", "")
		s.put(t, "dir/a/b.txt", "b")
		d := s.disk(t, container.Bag{s3.ConfigFieldPrefix: "root"})
		testutil.NoError(t, d.RemoveAll("/"))
		testutil.NoError(t, afero.WriteFile(d, "file.txt", []byte("hello"), 0o644))
		testutil.NoError(t, d.MkdirAll("dir", 0o755))
		testutil.NoError(t, afero.WriteFile(d, "dir/a/b.txt", []byte("b"), 0o644))

		testutil.NoError(t, d.Rename("file.txt", "moved.txt"))
		testutil.NoError(t, d.Rename("dir", "other/dir"))
		testutil.NoError(t, d.Rename("moved.txt", "/moved.txt"))
		testutil.NoError(t, d.Rename("other", "./other/"))
		testutil.Equal(t, "keys", s.keys(t), []string{
			"dir/", "dir/a/b.txt", "file.txt", "root/moved.txt", "root/other/dir/", "root/other/dir/a/b.txt",
		})
		testutil.Equal(t, "content", s.content(t, "root/moved.txt"), "hello")
	})

	t.Run("should fail on invalid renames", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		s.put(t, "dir/a.txt", "a")
		d := s.disk(t, nil)

		failed(t, d.Rename("missing", "other"), "rename", fs.ErrNotExist)
		failed(t, d.Rename("/", "other"), "rename", fs.ErrInvalid)
		failed(t, d.Rename("file.txt", "/"), "rename", fs.ErrInvalid)
		failed(t, d.Rename("dir", "dir/sub"), "rename", fs.ErrInvalid)
		testutil.Equal(t, "keys", s.keys(t), []string{"dir/a.txt", "file.txt"})
	})

	t.Run("should fail to rename when the storage fails", func(t *testing.T) {
		s := newServer(t)
		s.put(t, "file.txt", "hello")
		s.put(t, "dir/a.txt", "a")
		d := s.disk(t, nil)

		s.failOn(http.MethodHead, "", "")
		denied(t, d.Rename("file.txt", "other.txt"), "rename")
		s.failOn(http.MethodGet, "file.txt", "")
		denied(t, d.Rename("file.txt", "other.txt"), "rename")
		s.failOn(http.MethodPut, "", "")
		denied(t, d.Rename("file.txt", "other.txt"), "rename")
		s.failOn(http.MethodDelete, "", "")
		denied(t, d.Rename("file.txt", "other.txt"), "rename")
		s.failWhen(recursiveList)
		denied(t, d.Rename("dir", "other"), "rename")
		s.failOn(http.MethodPut, "", "")
		denied(t, d.Rename("dir", "other"), "rename")
		s.reset()
		testutil.Equal(t, "keys", s.keys(t), []string{"dir/a.txt", "file.txt", "other.txt"})
	})
}
