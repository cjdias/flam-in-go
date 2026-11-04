package exporters

import (
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters/none"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters/otlphttp"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// create returns the exporter "main" of the fields, created by the
// exporter factory of an app with the tracing bundle and the exporters
// bundle, the defaults overridden by the values.
func create(t *testing.T, fields container.Bag, values map[string]any) (flam.ITracingExporter, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), tracing.NewBundle(), exporters.NewBundle(), none.NewBundle(), otlphttp.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(tracing.ExporterConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	// the exporters bundle registers the drivers itself
	for _, b := range bundles[:3] {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var exporter flam.ITracingExporter
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ITracingExporterFactory) { exporter, err = f.Get("main") }))
	return exporter, err
}

// export exports a span named "work" through the exporter, at once.
func export(t *testing.T, exporter flam.ITracingExporter) {
	t.Helper()
	p := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	_, span := p.Tracer("test").Start(context.Background(), "work")
	span.End()
	testutil.NoError(t, p.Shutdown(context.Background()))
}

// request is a request received by a collector.
type request struct {
	path     string
	header   http.Header
	body     []byte
	gzipped  bool
	readErrs error
}

// collector is an OTLP HTTP collector recording the requests it receives.
type collector struct {
	*httptest.Server

	mu       sync.Mutex
	requests []request
}

// newCollector starts a collector, closed at the end of the test.
func newCollector(t *testing.T) *collector {
	t.Helper()
	c := &collector{}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var reader io.Reader = req.Body
		gzipped := req.Header.Get("Content-Encoding") == "gzip"
		var readErr error
		if gzipped {
			gz, err := gzip.NewReader(req.Body)
			readErr = err
			reader = gz
		}
		var body []byte
		if readErr == nil {
			body, readErr = io.ReadAll(reader)
		}
		c.mu.Lock()
		c.requests = append(c.requests, request{req.URL.Path, req.Header.Clone(), body, gzipped, readErr})
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(c.Close)
	return c
}

// received returns the requests received.
func (c *collector) received() []request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]request(nil), c.requests...)
}

// host returns the host and the port of the collector.
func (c *collector) host() string {
	return strings.TrimPrefix(c.URL, "http://")
}

func Test_Bundles(t *testing.T) {
	t.Run("should register every driver", func(t *testing.T) {
		b := exporters.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.tracing.exporters")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.tracing.exporters",
			"bundle:flam.tracing.exporters.none", "provider/1",
			"bundle:flam.tracing.exporters.otlphttp", "provider/1",
		})
	})

	t.Run("should configure the none exporter", func(t *testing.T) {
		b := none.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.tracing.exporters.none")
		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag, container.Bag{"flam": container.Bag{"tracing": container.Bag{
			"exporters": container.Bag{none.Id: container.Bag{factory.ConfigFieldDriver: none.Driver}},
		}}})
		testutil.Equal(t, "Id", none.Id, "none")
		testutil.Equal(t, "Driver", none.Driver, "flam.tracing.exporters.driver.none")
	})

	t.Run("should configure the otlphttp exporter", func(t *testing.T) {
		b := otlphttp.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.tracing.exporters.otlphttp")
		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			tracing.ExporterConfigPath + "." + otlphttp.Id: container.Bag{factory.ConfigFieldDriver: otlphttp.Driver},
			otlphttp.DefaultEndpointConfigPath:             "",
			otlphttp.DefaultPathConfigPath:                 "",
			otlphttp.DefaultInsecureConfigPath:             false,
			otlphttp.DefaultTimeoutConfigPath:              "10s",
			otlphttp.DefaultCompressionConfigPath:          "none",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "headers", bag.Has(otlphttp.DefaultHeadersConfigPath), false)
		testutil.Equal(t, "Id", otlphttp.Id, "otlphttp")
		testutil.Equal(t, "Driver", otlphttp.Driver, "flam.tracing.exporters.driver.otlphttp")
	})

	t.Run("should fail when a driver cannot be registered", func(t *testing.T) {
		for _, b := range []flam.IBundle{none.NewBundle(), otlphttp.NewBundle()} {
			app := testutil.NewApp()
			app.Fail = func(kind string) error {
				if kind == "provider" {
					return errors.New("boom")
				}
				return nil
			}
			if err := app.RegisterBundle(b); err == nil {
				t.Errorf("%s: RegisterBundle() = nil, want an error", b.Id())
			}
		}
	})

	t.Run("should fail to configure on a bag refusing the values", func(t *testing.T) {
		for _, b := range []flam.IBundle{none.NewBundle(), otlphttp.NewBundle()} {
			bag := container.Bag{"flam": "taken"}
			if err := b.(flam.IConfigurableBundle).Config(bag); err == nil {
				t.Errorf("%s: Config() = nil, want an error", b.Id())
			}
		}
	})
}

func Test_None(t *testing.T) {
	t.Run("should discard the spans", func(t *testing.T) {
		exporter, err := create(t, container.Bag{factory.ConfigFieldDriver: none.Driver}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", exporter.Id(), "main")
		export(t, exporter)
		testutil.NoError(t, exporter.ExportSpans(context.Background(), nil))
		testutil.NoError(t, exporter.Shutdown(context.Background()))
	})
}

func Test_OTLPHTTP(t *testing.T) {
	t.Run("should send the spans to the collector", func(t *testing.T) {
		c := newCollector(t)
		exporter, err := create(t, container.Bag{
			factory.ConfigFieldDriver:       otlphttp.Driver,
			otlphttp.ConfigFieldEndpoint:    c.host(),
			otlphttp.ConfigFieldPath:        "/custom/traces",
			otlphttp.ConfigFieldInsecure:    true,
			otlphttp.ConfigFieldHeaders:     container.Bag{"X-Token": "secret"},
			otlphttp.ConfigFieldTimeout:     5 * time.Second,
			otlphttp.ConfigFieldCompression: otlphttp.CompressionGzip,
		}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", exporter.Id(), "main")
		export(t, exporter)

		received := c.received()
		testutil.Equal(t, "requests", len(received), 1)
		testutil.Equal(t, "path", received[0].path, "/custom/traces")
		testutil.Equal(t, "token", received[0].header.Get("X-Token"), "secret")
		testutil.Equal(t, "gzipped", received[0].gzipped, true)
		testutil.NoError(t, received[0].readErrs)
		if !strings.Contains(string(received[0].body), "work") {
			t.Errorf("body = %q, want it to hold the span name", received[0].body)
		}
	})

	t.Run("should send to the defaults", func(t *testing.T) {
		c := newCollector(t)
		exporter, err := create(t, container.Bag{factory.ConfigFieldDriver: otlphttp.Driver}, map[string]any{
			otlphttp.DefaultEndpointConfigPath: c.host(),
			otlphttp.DefaultInsecureConfigPath: true,
			otlphttp.DefaultHeadersConfigPath:  map[string]any{"X-Default": "yes"},
		})
		testutil.NoError(t, err)
		export(t, exporter)

		received := c.received()
		testutil.Equal(t, "requests", len(received), 1)
		testutil.Equal(t, "path", received[0].path, "/v1/traces")
		testutil.Equal(t, "header", received[0].header.Get("X-Default"), "yes")
		testutil.Equal(t, "gzipped", received[0].gzipped, false)
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		for field, value := range map[string]any{
			otlphttp.ConfigFieldEndpoint:    1,
			otlphttp.ConfigFieldPath:        true,
			otlphttp.ConfigFieldInsecure:    "yes",
			otlphttp.ConfigFieldHeaders:     "X-Token: secret",
			otlphttp.ConfigFieldTimeout:     "0s",
			otlphttp.ConfigFieldCompression: "zstd",
		} {
			_, err := create(t, container.Bag{factory.ConfigFieldDriver: otlphttp.Driver, field: value}, nil)
			err = errors.Unwrap(err) // the error of the creator, wrapped by the factory
			testutil.Error(t, err, otlphttp.ErrInvalidField, "unable to create otlphttp tracing exporter: invalid otlphttp tracing exporter field",
				map[string]any{otlphttp.ErrorFieldID: "main", otlphttp.ErrorFieldField: field})
		}
	})

	t.Run("should fail on invalid headers", func(t *testing.T) {
		for name, headers := range map[string]any{
			"number": container.Bag{"X-Retries": 3},
			"list":   []string{"X-Token"},
			"nested": map[string]any{"X": map[string]any{"Y": "z"}},
		} {
			_, err := create(t, container.Bag{factory.ConfigFieldDriver: otlphttp.Driver, otlphttp.ConfigFieldHeaders: headers}, nil)
			if !errors.Is(err, otlphttp.ErrInvalidField) {
				t.Errorf("%s: error = %v, want ErrInvalidField", name, err)
			}
		}
	})

	t.Run("should fail when the exporter cannot be started", func(t *testing.T) {
		t.Setenv("OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE", certificate(t))
		_, err := create(t, container.Bag{
			factory.ConfigFieldDriver:    otlphttp.Driver,
			otlphttp.ConfigFieldInsecure: true,
		}, nil)
		err = errors.Unwrap(err) // the error of the creator, wrapped by the factory
		testutil.Error(t, err, otlphttp.ErrExporter, err.Error(), map[string]any{otlphttp.ErrorFieldID: "main"})
		if !strings.HasPrefix(err.Error(), "unable to create otlphttp tracing exporter: unable to start otlphttp tracing exporter: ") {
			t.Errorf("error = %q, want the message of the creator and the sentinel", err)
		}
	})
}

// certificate returns the path of the PEM file of a new self-signed
// certificate.
func certificate(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testutil.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	testutil.NoError(t, err)
	path := filepath.Join(t.TempDir(), "ca.pem")
	testutil.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	return path
}
