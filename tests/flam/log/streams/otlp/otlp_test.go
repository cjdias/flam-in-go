package otlp

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"go.uber.org/dig"
	"google.golang.org/protobuf/proto"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/otlp"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var errBoom = errors.New("boom")

// ts is the timestamp of the written entries.
var ts = time.Date(2026, 11, 2, 9, 43, 0, 0, time.UTC)

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// newApp returns a testutil.App with the config and log bundles, the
// bundle given, and a configuration holding their defaults, then the
// values, at paths.
func newApp(t *testing.T, values map[string]any, b flam.IBundle) *testutil.App {
	t.Helper()
	all := []flam.IBundle{config.NewBundle(), log.NewBundle(), b}
	bag := container.Bag{}
	for _, b := range all {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	for _, b := range all {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app
}

// creator returns the single stream creator of the app.
func creator(t *testing.T, app *testutil.App) flam.ILogStreamCreator {
	t.Helper()
	var creators []flam.ILogStreamCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.ILogStreamCreator `group:"flam.log.streams.creator"`
	}) {
		creators = args.Creators
	}))
	testutil.Equal(t, "creators", len(creators), 1)
	return creators[0]
}

// request is an export request received by a collector.
type request struct {
	path   string
	header http.Header
	body   *collogspb.ExportLogsServiceRequest
}

// collector is an OTLP/HTTP collector receiving the export requests.
type collector struct {
	t        *testing.T
	server   *httptest.Server
	requests chan request
}

// newCollector returns a collector answering every request with success.
func newCollector(t *testing.T) *collector {
	t.Helper()
	c := &collector{t: t, requests: make(chan request, 16)}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		body := &collogspb.ExportLogsServiceRequest{}
		if err == nil {
			err = proto.Unmarshal(data, body)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		c.requests <- request{path: r.URL.Path, header: r.Header.Clone(), body: body}
		response, _ := proto.Marshal(&collogspb.ExportLogsServiceResponse{})
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(response)
	}))
	t.Cleanup(c.server.Close)
	return c
}

// endpoint returns the host and port of the collector.
func (c *collector) endpoint() string {
	return strings.TrimPrefix(c.server.URL, "http://")
}

// next returns the next request received.
func (c *collector) next() request {
	c.t.Helper()
	select {
	case r := <-c.requests:
		return r
	case <-time.After(10 * time.Second):
		c.t.Fatal("no export request received")
		return request{}
	}
}

// none fails the test when a request was received.
func (c *collector) none() {
	c.t.Helper()
	select {
	case r := <-c.requests:
		c.t.Fatalf("request = %v, want none", r.body)
	default:
	}
}

// records returns the records of the request, and the attributes of the
// resource of the first ones.
func records(r request) ([]*logspb.LogRecord, map[string]any) {
	var all []*logspb.LogRecord
	var resource map[string]any
	for _, rl := range r.body.GetResourceLogs() {
		if resource == nil {
			resource = attributes(rl.GetResource().GetAttributes())
		}
		for _, sl := range rl.GetScopeLogs() {
			all = append(all, sl.GetLogRecords()...)
		}
	}
	return all, resource
}

// attributes returns the attributes as plain values.
func attributes(kvs []*commonpb.KeyValue) map[string]any {
	m := map[string]any{}
	for _, kv := range kvs {
		m[kv.GetKey()] = plain(kv.GetValue())
	}
	return m
}

// plain returns the value as a string, bool, int64, float64, []byte,
// []any, map[string]any, or nil for an empty value.
func plain(v *commonpb.AnyValue) any {
	switch value := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return value.StringValue
	case *commonpb.AnyValue_BoolValue:
		return value.BoolValue
	case *commonpb.AnyValue_IntValue:
		return value.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return value.DoubleValue
	case *commonpb.AnyValue_BytesValue:
		return value.BytesValue
	case *commonpb.AnyValue_ArrayValue:
		items := []any{}
		for _, item := range value.ArrayValue.GetValues() {
			items = append(items, plain(item))
		}
		return items
	case *commonpb.AnyValue_KvlistValue:
		return attributes(value.KvlistValue.GetValues())
	}
	return nil
}

// streamConfig returns the configuration of an otlp stream of id "s"
// holding the fields.
func streamConfig(fields ...container.Bag) container.Bag {
	bag := container.Bag{factory.ConfigFieldId: "s", factory.ConfigFieldDriver: otlp.Driver}
	for _, f := range fields {
		for k, v := range f {
			bag[k] = v
		}
	}
	return bag
}

// local returns the fields of a stream exporting to the collector.
func (c *collector) local() container.Bag {
	return container.Bag{otlp.ConfigFieldEndpoint: c.endpoint(), otlp.ConfigFieldInsecure: true}
}

// named is a string of a named type.
type named string

// flag is a bool of a named type.
type flag bool

// point is a struct, formatted by fmt.
type point struct{ X, Y int }

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should declare the driver, fields and paths", func(t *testing.T) {
		testutil.Equal(t, "Driver", otlp.Driver, "flam.log.streams.driver.otlp")
		testutil.Equal(t, "ScopeName", otlp.ScopeName, "github.com/cjdias/flam-in-go/pkg/flam/log")
		for _, c := range [][2]string{
			{otlp.ConfigFieldLevel, "level"},
			{otlp.ConfigFieldChannels, "channels"},
			{otlp.ConfigFieldEndpoint, "endpoint"},
			{otlp.ConfigFieldInsecure, "insecure"},
			{otlp.ConfigFieldHeaders, "headers"},
			{otlp.ConfigFieldTimeout, "timeout"},
			{otlp.ConfigFieldServiceName, "service_name"},
			{otlp.DefaultLevelConfigPath, "flam.log.defaults.streams.otlp.level"},
			{otlp.DefaultEndpointConfigPath, "flam.log.defaults.streams.otlp.endpoint"},
			{otlp.DefaultInsecureConfigPath, "flam.log.defaults.streams.otlp.insecure"},
			{otlp.DefaultHeadersConfigPath, "flam.log.defaults.streams.otlp.headers"},
			{otlp.DefaultTimeoutConfigPath, "flam.log.defaults.streams.otlp.timeout"},
			{otlp.DefaultServiceNameConfigPath, "flam.log.defaults.streams.otlp.service_name"},
			{otlp.ErrorFieldID, "id"},
			{otlp.ErrorFieldField, "field"},
			{otlp.ErrorFieldEndpoint, "endpoint"},
		} {
			testutil.Equal(t, "constant", c[0], c[1])
		}
	})

	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", otlp.NewBundle().Id(), "flam.log.streams.otlp")
	})

	t.Run("should set the defaults of the fields, and configure no stream", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, otlp.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "level", bag.Get(otlp.DefaultLevelConfigPath), "info")
		testutil.Equal(t, "endpoint", bag.Get(otlp.DefaultEndpointConfigPath), "localhost:4318")
		testutil.Equal(t, "timeout", bag.Get(otlp.DefaultTimeoutConfigPath), "10s")
		testutil.Equal(t, "insecure", bag.Has(otlp.DefaultInsecureConfigPath), false)
		testutil.Equal(t, "streams", bag.Has(log.StreamConfigPath), false)
	})

	t.Run("should fail to configure a bag refusing the defaults", func(t *testing.T) {
		if err := otlp.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": 1}); err == nil {
			t.Fatal("Config() = nil, want an error")
		}
	})

	t.Run("should register its creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(otlp.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.log.streams.otlp", "provider/1"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := otlp.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept only its driver", func(t *testing.T) {
		c := creator(t, newApp(t, nil, otlp.NewBundle()))
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": otlp.Driver}), true)
		testutil.Equal(t, "Accept(other)", c.Accept(container.Bag{"driver": "other"}), false)
	})

	t.Run("should export the records to the collector when closed", func(t *testing.T) {
		col := newCollector(t)
		s, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(col.local(), container.Bag{
			otlp.ConfigFieldHeaders:     container.Bag{"authorization": "Bearer t"},
			otlp.ConfigFieldServiceName: "svc",
			otlp.ConfigFieldChannels:    []string{"a"},
			otlp.ConfigFieldTimeout:     "5s",
		}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", s.Id(), "s")
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)

		testutil.NoError(t, s.Broadcast(ts, flam.LogDebug, "hidden"))
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "b", "hidden"))
		testutil.NoError(t, s.Broadcast(ts, flam.LogWarning, "first", container.Bag{"k": "v"}, nil))
		testutil.NoError(t, s.Signal(ts.Add(time.Second), flam.LogError, "a", "second"))
		testutil.NoError(t, s.Close())

		r := col.next()
		testutil.Equal(t, "path", r.path, "/v1/logs")
		testutil.Equal(t, "authorization", r.header.Get("Authorization"), "Bearer t")
		testutil.Equal(t, "content type", r.header.Get("Content-Type"), "application/x-protobuf")
		got, resource := records(r)
		testutil.Equal(t, "service.name", resource["service.name"], "svc")
		testutil.Equal(t, "telemetry.sdk.language", resource["telemetry.sdk.language"], "go")
		testutil.Equal(t, "records", len(got), 2)
		testutil.Equal(t, "scope", r.body.GetResourceLogs()[0].GetScopeLogs()[0].GetScope().GetName(), otlp.ScopeName)

		testutil.Equal(t, "time", got[0].GetTimeUnixNano(), uint64(ts.UnixNano()))
		testutil.Equal(t, "severity", got[0].GetSeverityNumber(), logspb.SeverityNumber_SEVERITY_NUMBER_WARN)
		testutil.Equal(t, "severity text", got[0].GetSeverityText(), "WARNING")
		testutil.Equal(t, "body", plain(got[0].GetBody()), "first")
		testutil.Equal(t, "attributes", attributes(got[0].GetAttributes()), map[string]any{"k": "v"})
		if got[0].GetObservedTimeUnixNano() == 0 {
			t.Error("observed time = 0, want it set")
		}

		testutil.Equal(t, "time", got[1].GetTimeUnixNano(), uint64(ts.Add(time.Second).UnixNano()))
		testutil.Equal(t, "severity", got[1].GetSeverityNumber(), logspb.SeverityNumber_SEVERITY_NUMBER_ERROR)
		testutil.Equal(t, "severity text", got[1].GetSeverityText(), "ERROR")
		testutil.Equal(t, "body", plain(got[1].GetBody()), "second")
		testutil.Equal(t, "attributes", attributes(got[1].GetAttributes()), map[string]any{log.ContextFieldChannel: "a"})
		col.none()
	})

	t.Run("should map the levels to the severities", func(t *testing.T) {
		col := newCollector(t)
		s, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(col.local(), container.Bag{
			otlp.ConfigFieldLevel: flam.LogDebug,
		}))
		testutil.NoError(t, err)
		levels := []flam.LogLevel{flam.LogFatal, flam.LogError, flam.LogWarning, flam.LogNotice, flam.LogInfo, flam.LogDebug}
		for _, level := range levels {
			testutil.NoError(t, s.Broadcast(ts, level, level.String()))
		}
		testutil.NoError(t, s.Close())

		got, _ := records(col.next())
		want := []logspb.SeverityNumber{
			logspb.SeverityNumber_SEVERITY_NUMBER_FATAL, logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
			logspb.SeverityNumber_SEVERITY_NUMBER_WARN, logspb.SeverityNumber_SEVERITY_NUMBER_INFO2,
			logspb.SeverityNumber_SEVERITY_NUMBER_INFO, logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG,
		}
		testutil.Equal(t, "records", len(got), len(levels))
		for i, level := range levels {
			testutil.Equal(t, "severity of "+level.String(), got[i].GetSeverityNumber(), want[i])
			testutil.Equal(t, "severity text of "+level.String(), got[i].GetSeverityText(), strings.ToUpper(level.String()))
			testutil.Equal(t, "body of "+level.String(), plain(got[i].GetBody()), level.String())
		}
	})

	t.Run("should convert the context values to attributes", func(t *testing.T) {
		col := newCollector(t)
		s, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(col.local()))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m", container.Bag{
			"nil":      nil,
			"string":   "s",
			"bool":     true,
			"bytes":    []byte("b"),
			"time":     time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC),
			"error":    errBoom,
			"stringer": 90 * time.Second,
			"bag":      container.Bag{"x": 1, "y": container.Bag{"z": "w"}},
			"named":    named("n"),
			"flag":     flag(true),
			"int":      -3,
			"int8":     int8(8),
			"uint":     uint(4),
			"uint64":   uint64(math.MaxUint64),
			"float32":  float32(1.5),
			"float64":  2.25,
			"slice":    []any{1, "two", nil},
			"array":    [2]string{"a", "b"},
			"map":      map[string]int{"b": 2, "a": 1},
			"intmap":   map[int]string{1: "one"},
			"struct":   point{1, 2},
		}))
		testutil.NoError(t, s.Close())

		got, _ := records(col.next())
		testutil.Equal(t, "attributes", attributes(got[0].GetAttributes()), map[string]any{
			"nil":      nil,
			"string":   "s",
			"bool":     true,
			"bytes":    []byte("b"),
			"time":     "2026-01-02T03:04:05.000000006Z",
			"error":    "boom",
			"stringer": "1m30s",
			"bag":      map[string]any{"x": int64(1), "y": map[string]any{"z": "w"}},
			"named":    "n",
			"flag":     true,
			"int":      int64(-3),
			"int8":     int64(8),
			"uint":     int64(4),
			"uint64":   fmt.Sprint(uint64(math.MaxUint64)),
			"float32":  1.5,
			"float64":  2.25,
			"slice":    []any{int64(1), "two", nil},
			"array":    []any{"a", "b"},
			"map":      map[string]any{"a": int64(1), "b": int64(2)},
			"intmap":   "map[1:one]",
			"struct":   "{1 2}",
		})
	})

	t.Run("should take the headers of a map", func(t *testing.T) {
		for _, headers := range []any{map[string]string{"x-key": "k"}, map[string]any{"x-key": "k"}} {
			col := newCollector(t)
			s, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(col.local(), container.Bag{
				otlp.ConfigFieldHeaders: headers,
			}))
			testutil.NoError(t, err)
			testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
			testutil.NoError(t, s.Close())
			testutil.Equal(t, "header", col.next().header.Get("X-Key"), "k")
		}
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		col := newCollector(t)
		app := newApp(t, map[string]any{
			otlp.DefaultLevelConfigPath:       2,
			otlp.DefaultEndpointConfigPath:    col.endpoint(),
			otlp.DefaultInsecureConfigPath:    true,
			otlp.DefaultHeadersConfigPath:     container.Bag{"x-key": "d"},
			otlp.DefaultTimeoutConfigPath:     time.Second,
			otlp.DefaultServiceNameConfigPath: "default",
		}, otlp.NewBundle())
		s, err := creator(t, app).Create(streamConfig())
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogError)
		testutil.NoError(t, s.Broadcast(ts, flam.LogError, "m"))
		testutil.NoError(t, s.Close())
		r := col.next()
		testutil.Equal(t, "header", r.header.Get("X-Key"), "d")
		_, resource := records(r)
		testutil.Equal(t, "service.name", resource["service.name"], "default")
	})

	t.Run("should take the built-in defaults", func(t *testing.T) {
		s, err := creator(t, newApp(t, nil, bare{otlp.NewBundle()})).Create(streamConfig())
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		testutil.NoError(t, s.Close())
	})

	t.Run("should leave the default service name of the sdk", func(t *testing.T) {
		col := newCollector(t)
		s, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(col.local()))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		_, resource := records(col.next())
		name, _ := resource["service.name"].(string)
		if !strings.HasPrefix(name, "unknown_service") {
			t.Errorf("service.name = %q, want the default of the sdk", name)
		}
	})

	for name, fields := range map[string]container.Bag{
		"level":         {otlp.ConfigFieldLevel: 9},
		"channels":      {otlp.ConfigFieldChannels: 1},
		"endpoint":      {otlp.ConfigFieldEndpoint: ""},
		"endpoint ":     {otlp.ConfigFieldEndpoint: 1},
		"insecure":      {otlp.ConfigFieldInsecure: "yes"},
		"headers":       {otlp.ConfigFieldHeaders: 1},
		"headers ":      {otlp.ConfigFieldHeaders: container.Bag{"a": 1}},
		"headers  ":     {otlp.ConfigFieldHeaders: map[string]any{"a": 1}},
		"timeout":       {otlp.ConfigFieldTimeout: "0s"},
		"timeout ":      {otlp.ConfigFieldTimeout: "x"},
		"service_name":  {otlp.ConfigFieldServiceName: 1},
		"service_name ": {otlp.ConfigFieldServiceName: true},
	} {
		t.Run("should refuse an invalid "+name, func(t *testing.T) {
			_, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(fields))
			testutil.Error(t, err, otlp.ErrInvalidField,
				"unable to create otlp log stream: invalid otlp log stream field",
				map[string]any{otlp.ErrorFieldID: "s", otlp.ErrorFieldField: strings.TrimSpace(name)})
		})
	}

	t.Run("should fail for an endpoint that is not a host", func(t *testing.T) {
		_, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(container.Bag{
			otlp.ConfigFieldEndpoint: "bad host",
		}))
		testutil.Error(t, err, otlp.ErrExporter, err.Error(),
			map[string]any{otlp.ErrorFieldID: "s", otlp.ErrorFieldEndpoint: "bad host"})
		if !strings.HasPrefix(err.Error(), "unable to create otlp log stream: unable to create otlp log stream exporter: ") {
			t.Errorf("Error() = %q, want the exporter error", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

func Test_Stream(t *testing.T) {
	t.Run("should refuse the writes once closed", func(t *testing.T) {
		col := newCollector(t)
		s, err := creator(t, newApp(t, nil, otlp.NewBundle())).Create(streamConfig(col.local()))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		testutil.Error(t, s.Broadcast(ts, flam.LogInfo, "m"), log.ErrStreamClosed,
			"unable to write log entry: log stream closed", map[string]any{log.ErrorFieldID: "s"})
		col.none()
	})
}
