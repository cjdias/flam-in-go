//go:build !windows && !plan9

package syslog

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/syslog"
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

// newApp returns a testutil.App with the config, log and serializer
// bundles, the bundle given, and a configuration holding their defaults,
// then the values, at paths.
func newApp(t *testing.T, values map[string]any, b flam.IBundle) *testutil.App {
	t.Helper()
	all := []flam.IBundle{config.NewBundle(), log.NewBundle(), stringify.NewBundle(), json.NewBundle(), b}
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

// server is a syslog daemon listening on a local packet connection.
type server struct {
	t    *testing.T
	conn net.PacketConn
}

// listenUDP returns a server listening on a local UDP port.
func listenUDP(t *testing.T) *server {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &server{t: t, conn: conn}
}

// listenUnixgram returns a server listening on a unix datagram socket, in
// a directory of a short path, as the socket paths are limited.
func listenUnixgram(t *testing.T) *server {
	t.Helper()
	dir, err := os.MkdirTemp("", "syslog")
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	conn, err := net.ListenPacket("unixgram", filepath.Join(dir, "s"))
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &server{t: t, conn: conn}
}

// fields returns the network and address fields of the server.
func (s *server) fields() container.Bag {
	return container.Bag{
		syslog.ConfigFieldNetwork: s.conn.LocalAddr().Network(),
		syslog.ConfigFieldAddress: s.conn.LocalAddr().String(),
	}
}

// read returns the next message received.
func (s *server) read() string {
	s.t.Helper()
	testutil.NoError(s.t, s.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	buf := make([]byte, 4096)
	n, _, err := s.conn.ReadFrom(buf)
	testutil.NoError(s.t, err)
	return string(buf[:n])
}

// message matches a message sent by the streams, capturing its priority,
// tag and content: the timestamp and hostname are skipped, the local
// format having a timestamp with spaces and no hostname.
var message = regexp.MustCompile(`^<(\d+)>.*? (\S*)\[\d+\]: (.*)$`)

// parse returns the priority, tag and content of the message, failing the
// test when it is not one.
func parse(t *testing.T, msg string) (string, string, string) {
	t.Helper()
	m := message.FindStringSubmatch(strings.TrimSuffix(msg, "\n"))
	if m == nil {
		t.Fatalf("message = %q, want a syslog message", msg)
	}
	return m[1], m[2], m[3]
}

// streamConfig returns the configuration of a syslog stream of id "s"
// holding the fields.
func streamConfig(fields ...container.Bag) container.Bag {
	bag := container.Bag{factory.ConfigFieldId: "s", factory.ConfigFieldDriver: syslog.Driver}
	for _, f := range fields {
		for k, v := range f {
			bag[k] = v
		}
	}
	return bag
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should declare the driver, fields and paths", func(t *testing.T) {
		testutil.Equal(t, "Driver", syslog.Driver, "flam.log.streams.driver.syslog")
		for _, c := range [][2]string{
			{syslog.ConfigFieldLevel, "level"},
			{syslog.ConfigFieldChannels, "channels"},
			{syslog.ConfigFieldSerializer, "serializer"},
			{syslog.ConfigFieldNetwork, "network"},
			{syslog.ConfigFieldAddress, "address"},
			{syslog.ConfigFieldTag, "tag"},
			{syslog.ConfigFieldFacility, "facility"},
			{syslog.DefaultLevelConfigPath, "flam.log.defaults.streams.syslog.level"},
			{syslog.DefaultSerializerConfigPath, "flam.log.defaults.streams.syslog.serializer"},
			{syslog.DefaultNetworkConfigPath, "flam.log.defaults.streams.syslog.network"},
			{syslog.DefaultAddressConfigPath, "flam.log.defaults.streams.syslog.address"},
			{syslog.DefaultTagConfigPath, "flam.log.defaults.streams.syslog.tag"},
			{syslog.DefaultFacilityConfigPath, "flam.log.defaults.streams.syslog.facility"},
			{syslog.ErrorFieldID, "id"},
			{syslog.ErrorFieldField, "field"},
			{syslog.ErrorFieldSerializer, "serializer"},
			{syslog.ErrorFieldNetwork, "network"},
			{syslog.ErrorFieldAddress, "address"},
		} {
			testutil.Equal(t, "constant", c[0], c[1])
		}
	})

	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", syslog.NewBundle().Id(), "flam.log.streams.syslog")
	})

	t.Run("should set the defaults of the fields, and configure no stream", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, syslog.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "level", bag.Get(syslog.DefaultLevelConfigPath), "info")
		testutil.Equal(t, "serializer", bag.Get(syslog.DefaultSerializerConfigPath), stringify.Id)
		testutil.Equal(t, "facility", bag.Get(syslog.DefaultFacilityConfigPath), "user")
		testutil.Equal(t, "network", bag.Has(syslog.DefaultNetworkConfigPath), false)
		testutil.Equal(t, "streams", bag.Has(log.StreamConfigPath), false)
	})

	t.Run("should fail to configure a bag refusing the defaults", func(t *testing.T) {
		if err := syslog.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": 1}); err == nil {
			t.Fatal("Config() = nil, want an error")
		}
	})

	t.Run("should register its creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(syslog.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.log.streams.syslog", "provider/1"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := syslog.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept only its driver", func(t *testing.T) {
		c := creator(t, newApp(t, nil, syslog.NewBundle()))
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": syslog.Driver}), true)
		testutil.Equal(t, "Accept(other)", c.Accept(container.Bag{"driver": "other"}), false)
	})

	t.Run("should send to a udp daemon with the defaults", func(t *testing.T) {
		srv := listenUDP(t)
		s, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(streamConfig(srv.fields()))
		testutil.NoError(t, err)
		defer func() { _ = s.Close() }()
		testutil.Equal(t, "Id()", s.Id(), "s")
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)

		testutil.NoError(t, s.Broadcast(ts, flam.LogDebug, "hidden"))
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		priority, tag, content := parse(t, srv.read())
		testutil.Equal(t, "priority", priority, "14")
		testutil.Equal(t, "tag", tag, os.Args[0])
		testutil.Equal(t, "content", content, "2026-11-02T09:43:00.000+0000 [INFO] m")
	})

	t.Run("should send to a unix datagram daemon", func(t *testing.T) {
		srv := listenUnixgram(t)
		s, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(streamConfig(srv.fields(), container.Bag{
			syslog.ConfigFieldTag: "app", syslog.ConfigFieldSerializer: json.Id,
			syslog.ConfigFieldChannels: []string{"a"}, syslog.ConfigFieldFacility: "LOCAL0",
		}))
		testutil.NoError(t, err)
		defer func() { _ = s.Close() }()
		testutil.NoError(t, s.Signal(ts, flam.LogError, "b", "hidden"))
		testutil.NoError(t, s.Signal(ts, flam.LogError, "a", "m"))
		priority, tag, content := parse(t, srv.read())
		testutil.Equal(t, "priority", priority, "131")
		testutil.Equal(t, "tag", tag, "app")
		testutil.Equal(t, "content", content,
			`{"channel":"a","level":"ERROR","message":"m","time":"2026-11-02T09:43:00.000+0000"}`)
	})

	t.Run("should map the levels to the severities", func(t *testing.T) {
		srv := listenUDP(t)
		s, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(streamConfig(srv.fields(), container.Bag{
			syslog.ConfigFieldLevel: "debug", syslog.ConfigFieldFacility: "daemon",
		}))
		testutil.NoError(t, err)
		defer func() { _ = s.Close() }()
		for level, priority := range map[flam.LogLevel]string{
			flam.LogFatal: "26", flam.LogError: "27", flam.LogWarning: "28",
			flam.LogNotice: "29", flam.LogInfo: "30", flam.LogDebug: "31",
		} {
			testutil.NoError(t, s.Broadcast(ts, level, "m"))
			got, _, content := parse(t, srv.read())
			testutil.Equal(t, "priority of "+level.String(), got, priority)
			testutil.Equal(t, "content of "+level.String(), content,
				"2026-11-02T09:43:00.000+0000 ["+strings.ToUpper(level.String())+"] m")
		}
	})

	t.Run("should take every facility", func(t *testing.T) {
		srv := listenUDP(t)
		c := creator(t, newApp(t, nil, syslog.NewBundle()))
		for i, facility := range []string{
			"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news", "uucp", "cron", "authpriv", "ftp",
		} {
			s, err := c.Create(streamConfig(srv.fields(), container.Bag{syslog.ConfigFieldFacility: facility}))
			testutil.NoError(t, err)
			testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
			priority, _, _ := parse(t, srv.read())
			testutil.Equal(t, "priority of "+facility, priority, strconv.Itoa(i*8+6))
			testutil.NoError(t, s.Close())
		}
		for i := range 8 {
			facility := "local" + strconv.Itoa(i)
			s, err := c.Create(streamConfig(srv.fields(), container.Bag{syslog.ConfigFieldFacility: facility}))
			testutil.NoError(t, err)
			testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
			priority, _, _ := parse(t, srv.read())
			testutil.Equal(t, "priority of "+facility, priority, strconv.Itoa((16+i)*8+6))
			testutil.NoError(t, s.Close())
		}
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		srv := listenUDP(t)
		app := newApp(t, map[string]any{
			syslog.DefaultLevelConfigPath: 2, syslog.DefaultSerializerConfigPath: json.Id,
			syslog.DefaultNetworkConfigPath: "udp", syslog.DefaultAddressConfigPath: srv.conn.LocalAddr().String(),
			syslog.DefaultTagConfigPath: "tagged", syslog.DefaultFacilityConfigPath: "mail",
		}, syslog.NewBundle())
		s, err := creator(t, app).Create(streamConfig())
		testutil.NoError(t, err)
		defer func() { _ = s.Close() }()
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogError)
		testutil.NoError(t, s.Broadcast(ts, flam.LogError, "m"))
		priority, tag, content := parse(t, srv.read())
		testutil.Equal(t, "priority", priority, "19")
		testutil.Equal(t, "tag", tag, "tagged")
		testutil.Equal(t, "content", content, `{"level":"ERROR","message":"m","time":"2026-11-02T09:43:00.000+0000"}`)
	})

	t.Run("should take the built-in defaults", func(t *testing.T) {
		srv := listenUDP(t)
		s, err := creator(t, newApp(t, nil, bare{syslog.NewBundle()})).Create(streamConfig(srv.fields()))
		testutil.NoError(t, err)
		defer func() { _ = s.Close() }()
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		priority, _, content := parse(t, srv.read())
		testutil.Equal(t, "priority", priority, "14")
		testutil.Equal(t, "content", content, "2026-11-02T09:43:00.000+0000 [INFO] m")
	})

	for name, fields := range map[string]container.Bag{
		"level":      {syslog.ConfigFieldLevel: 9},
		"channels":   {syslog.ConfigFieldChannels: 1},
		"serializer": {syslog.ConfigFieldSerializer: ""},
		"network":    {syslog.ConfigFieldNetwork: 1},
		"address":    {syslog.ConfigFieldAddress: true},
		"tag":        {syslog.ConfigFieldTag: []string{}},
		"facility":   {syslog.ConfigFieldFacility: "local8"},
		"facility ":  {syslog.ConfigFieldFacility: 1},
	} {
		t.Run("should refuse an invalid "+name, func(t *testing.T) {
			_, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(streamConfig(fields))
			testutil.Error(t, err, syslog.ErrInvalidField,
				"unable to create syslog log stream: invalid syslog log stream field",
				map[string]any{syslog.ErrorFieldID: "s", syslog.ErrorFieldField: strings.TrimSpace(name)})
		})
	}

	t.Run("should fail for an unknown serializer", func(t *testing.T) {
		_, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(
			streamConfig(container.Bag{syslog.ConfigFieldSerializer: "x"}))
		testutil.Error(t, err, syslog.ErrSerializer, err.Error(),
			map[string]any{syslog.ErrorFieldID: "s", syslog.ErrorFieldSerializer: "x"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Errorf("error = %v, want it to wrap ErrUnknownID", err)
		}
	})

	t.Run("should fail for a daemon it cannot connect to", func(t *testing.T) {
		_, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(streamConfig(container.Bag{
			syslog.ConfigFieldNetwork: "nope", syslog.ConfigFieldAddress: "x",
		}))
		testutil.Error(t, err, syslog.ErrDial, err.Error(), map[string]any{
			syslog.ErrorFieldID: "s", syslog.ErrorFieldNetwork: "nope", syslog.ErrorFieldAddress: "x",
		})
		if !strings.HasPrefix(err.Error(), "unable to create syslog log stream: unable to connect to syslog: ") {
			t.Errorf("Error() = %q, want the dial error", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

func Test_Stream(t *testing.T) {
	t.Run("should close the connection when closed", func(t *testing.T) {
		srv := listenUDP(t)
		s, err := creator(t, newApp(t, nil, syslog.NewBundle())).Create(streamConfig(srv.fields()))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		testutil.Error(t, s.Broadcast(ts, flam.LogInfo, "m"), log.ErrStreamClosed,
			"unable to write log entry: log stream closed", map[string]any{log.ErrorFieldID: "s"})
	})
}
