package connections

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gomongo "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo/connections/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// unreachable is the URI of a deployment that cannot be reached.
const unreachable = "mongodb://127.0.0.1:1/?directConnection=true"

// create returns the connection "main" of the standard driver and the
// fields, of an app with the mongo bundles and the values, its factory
// closed at the end of the test.
func create(t *testing.T, fields container.Bag, values map[string]any) (flam.IMongoConnection, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), mongo.NewBundle(), standard.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	fields[factory.ConfigFieldDriver] = standard.Driver
	testutil.NoError(t, bag.Set(mongo.ConnectionConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var f flam.IMongoConnectionFactory
	testutil.NoError(t, app.Invoke(func(provided flam.IMongoConnectionFactory) { f = provided }))
	t.Cleanup(func() { _ = f.Close() })
	return f.Get("main")
}

// assertError asserts that the creation failed with the sentinel and the
// context.
func assertError(t *testing.T, err, sentinel error, context map[string]any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(errors.Unwrap(err), &e) {
		t.Fatalf("Get() = %v, want %v", err, sentinel)
	}
	testutil.Context(t, e, context)
}

// certificate is a self-signed certificate of localhost, its own
// authority, for servers and clients, written to PEM files.
type certificate struct {
	tls      tls.Certificate
	pool     *x509.CertPool
	certFile string
	keyFile  string
}

// newCertificate returns a new certificate, its files in a temporary
// directory.
func newCertificate(t *testing.T) certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testutil.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		DNSNames:              []string{"localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	testutil.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	testutil.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	dir := t.TempDir()
	c := certificate{
		pool:     x509.NewCertPool(),
		certFile: filepath.Join(dir, "cert.pem"),
		keyFile:  filepath.Join(dir, "key.pem"),
	}
	testutil.NoError(t, os.WriteFile(c.certFile, certPEM, 0o600))
	testutil.NoError(t, os.WriteFile(c.keyFile, keyPEM, 0o600))
	c.tls, err = tls.X509KeyPair(certPEM, keyPEM)
	testutil.NoError(t, err)
	c.pool.AppendCertsFromPEM(certPEM)
	return c
}

// handshake is what a TLS server saw of the first client completing a
// handshake.
type handshake struct {
	serverName string
	peerCerts  int
	payload    []byte
}

// tlsServer starts a TLS server of the certificate, asking for client
// certificates of the authority when clientAuth says so, returning its
// URI and the channel of the first handshake completed, along with the
// first command the client sent, up to want. It stops at the end of the
// test.
func tlsServer(t *testing.T, cert certificate, clientAuth tls.ClientAuthType, want []byte) (string, <-chan handshake) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert.tls},
		ClientAuth:   clientAuth,
		ClientCAs:    cert.pool,
	})
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	done := make(chan handshake, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				tlsConn := conn.(*tls.Conn)
				_ = tlsConn.SetDeadline(time.Now().Add(5 * time.Second))
				if tlsConn.Handshake() != nil {
					return
				}
				state := tlsConn.ConnectionState()
				var payload []byte
				buf := make([]byte, 4096)
				for !bytes.Contains(payload, want) {
					n, err := tlsConn.Read(buf)
					payload = append(payload, buf[:n]...)
					if err != nil {
						break
					}
				}
				select {
				case done <- handshake{serverName: state.ServerName, peerCerts: len(state.PeerCertificates), payload: payload}:
				default:
				}
			}()
		}
	}()
	return "mongodb://" + ln.Addr().String() + "/?directConnection=true", done
}

// waitHandshake returns the first handshake of the server.
func waitHandshake(t *testing.T, done <-chan handshake) handshake {
	t.Helper()
	select {
	case h := <-done:
		return h
	case <-time.After(10 * time.Second):
		t.Fatal("no TLS handshake")
		return handshake{}
	}
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", standard.Driver, "flam.mongo.connections.driver.standard")
	testutil.Equal(t, "ErrorFieldID", standard.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", standard.ErrorFieldField, "field")
	for got, want := range map[string]string{
		standard.ConfigFieldURI:                    "uri",
		standard.ConfigFieldDatabase:               "database",
		standard.ConfigFieldAppName:                "app_name",
		standard.ConfigFieldUsername:               "username",
		standard.ConfigFieldPassword:               "password",
		standard.ConfigFieldAuthSource:             "auth_source",
		standard.ConfigFieldAuthMechanism:          "auth_mechanism",
		standard.ConfigFieldConnectTimeout:         "connect_timeout",
		standard.ConfigFieldServerSelectionTimeout: "server_selection_timeout",
		standard.ConfigFieldTimeout:                "timeout",
		standard.ConfigFieldHeartbeatInterval:      "heartbeat_interval",
		standard.ConfigFieldMaxConnIdleTime:        "max_conn_idle_time",
		standard.ConfigFieldMaxPoolSize:            "max_pool_size",
		standard.ConfigFieldMinPoolSize:            "min_pool_size",
		standard.ConfigFieldMaxConnecting:          "max_connecting",
		standard.ConfigFieldTLS:                    "tls",
		standard.ConfigFieldTLSCAFile:              "tls_ca_file",
		standard.ConfigFieldTLSCertFile:            "tls_cert_file",
		standard.ConfigFieldTLSKeyFile:             "tls_key_file",
		standard.ConfigFieldTLSServerName:          "tls_server_name",
		standard.ConfigFieldTLSInsecure:            "tls_insecure",
		standard.ConfigFieldPing:                   "ping",
		standard.DefaultURIConfigPath:              "flam.mongo.defaults.connections.standard.uri",
		standard.DefaultDatabaseConfigPath:         "flam.mongo.defaults.connections.standard.database",
		standard.DefaultAppNameConfigPath:          "flam.mongo.defaults.connections.standard.app_name",
		standard.DefaultUsernameConfigPath:         "flam.mongo.defaults.connections.standard.username",
		standard.DefaultPasswordConfigPath:         "flam.mongo.defaults.connections.standard.password",
		standard.DefaultPingConfigPath:             "flam.mongo.defaults.connections.standard.ping",
	} {
		testutil.Equal(t, "declaration", got, want)
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the creator and the defaults", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.mongo.connections.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			standard.DefaultURIConfigPath:      "mongodb://127.0.0.1:27017",
			standard.DefaultDatabaseConfigPath: "flam",
			standard.DefaultAppNameConfigPath:  "",
			standard.DefaultUsernameConfigPath: "",
			standard.DefaultPasswordConfigPath: "",
			standard.DefaultPingConfigPath:     false,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.mongo.connections.standard", "provider/1"})
	})

	t.Run("should fail to configure a bag that cannot be set", func(t *testing.T) {
		bag := container.Bag{"flam": "scalar"}
		if err := standard.NewBundle().(flam.IConfigurableBundle).Config(bag); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should create a connection of the defaults without reaching the deployment", func(t *testing.T) {
		conn, err := create(t, container.Bag{}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", conn.Id(), "main")
		testutil.Equal(t, "Database().Name()", conn.Database().Name(), "flam")
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		conn, err := create(t, container.Bag{}, map[string]any{
			standard.DefaultURIConfigPath:      unreachable,
			standard.DefaultDatabaseConfigPath: "other",
			standard.DefaultAppNameConfigPath:  "app",
			standard.DefaultUsernameConfigPath: "user",
			standard.DefaultPasswordConfigPath: "secret",
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "Database().Name()", conn.Database().Name(), "other")
	})

	t.Run("should create a connection of every field", func(t *testing.T) {
		conn, err := create(t, container.Bag{
			standard.ConfigFieldURI:                    unreachable,
			standard.ConfigFieldDatabase:               "app",
			standard.ConfigFieldAppName:                "app",
			standard.ConfigFieldUsername:               "user",
			standard.ConfigFieldPassword:               "secret",
			standard.ConfigFieldAuthSource:             "admin",
			standard.ConfigFieldAuthMechanism:          "SCRAM-SHA-256",
			standard.ConfigFieldConnectTimeout:         "1s",
			standard.ConfigFieldServerSelectionTimeout: time.Second,
			standard.ConfigFieldTimeout:                "2s",
			standard.ConfigFieldHeartbeatInterval:      "1s",
			standard.ConfigFieldMaxConnIdleTime:        "1m",
			standard.ConfigFieldMaxPoolSize:            10,
			standard.ConfigFieldMinPoolSize:            uint8(1),
			standard.ConfigFieldMaxConnecting:          2.0,
			standard.ConfigFieldTLS:                    false,
			standard.ConfigFieldTLSCAFile:              "missing.pem",
			standard.ConfigFieldPing:                   false,
		}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Database().Name()", conn.Database().Name(), "app")
	})

	t.Run("should ping the deployment at creation within the server selection timeout", func(t *testing.T) {
		start := time.Now()
		_, err := create(t, container.Bag{
			standard.ConfigFieldURI:                    unreachable,
			standard.ConfigFieldServerSelectionTimeout: "100ms",
			standard.ConfigFieldConnectTimeout:         "100ms",
			standard.ConfigFieldPing:                   true,
		}, nil)
		var e *flam.Error
		if !gomongo.IsTimeout(err) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want a server selection timeout", err)
		}
		testutil.Context(t, e, map[string]any{mongo.ErrorFieldID: "main"})
		testutil.Equal(t, "ping error", strings.HasPrefix(e.Error(), "unable to ping mongo connection: "), true)
		testutil.Equal(t, "within the server selection timeout", time.Since(start) < 10*time.Second, true)
	})

	t.Run("should fail with the client options the driver refuses", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{
			"uri":                {standard.ConfigFieldURI: "nope://host"},
			"heartbeat interval": {standard.ConfigFieldHeartbeatInterval: "100ms"},
			"pool sizes":         {standard.ConfigFieldMinPoolSize: 5, standard.ConfigFieldMaxPoolSize: 1},
			"auth mechanism":     {standard.ConfigFieldAuthMechanism: "NOPE"},
		} {
			_, err := create(t, fields, nil)
			if !strings.HasPrefix(errors.Unwrap(err).Error(), "unable to create standard mongo connection: unable to connect standard mongo connection: ") {
				t.Errorf("%s: Get() = %v, want the error of the driver", name, err)
			}
			assertError(t, err, standard.ErrConnect, map[string]any{standard.ErrorFieldID: "main"})
		}
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{standard.ConfigFieldURI, ""},
			{standard.ConfigFieldURI, 1},
			{standard.ConfigFieldDatabase, ""},
			{standard.ConfigFieldAppName, 1},
			{standard.ConfigFieldUsername, 1},
			{standard.ConfigFieldPassword, 1},
			{standard.ConfigFieldAuthSource, 1},
			{standard.ConfigFieldAuthMechanism, 1},
			{standard.ConfigFieldConnectTimeout, "-1s"},
			{standard.ConfigFieldServerSelectionTimeout, "soon"},
			{standard.ConfigFieldTimeout, 1},
			{standard.ConfigFieldHeartbeatInterval, "x"},
			{standard.ConfigFieldMaxConnIdleTime, "x"},
			{standard.ConfigFieldMaxPoolSize, -1},
			{standard.ConfigFieldMinPoolSize, 1.5},
			{standard.ConfigFieldMaxConnecting, "1"},
			{standard.ConfigFieldMaxPoolSize, uint64(math.MaxUint32) + 1},
			{standard.ConfigFieldTLS, "yes"},
			{standard.ConfigFieldTLSCAFile, 1},
			{standard.ConfigFieldTLSCertFile, 1},
			{standard.ConfigFieldTLSKeyFile, 1},
			{standard.ConfigFieldTLSServerName, 1},
			{standard.ConfigFieldTLSInsecure, "no"},
			{standard.ConfigFieldPing, "no"},
		} {
			_, err := create(t, container.Bag{tc.field: tc.value}, nil)
			testutil.Error(t, errors.Unwrap(err), standard.ErrInvalidField,
				"unable to create standard mongo connection: invalid standard mongo connection field", map[string]any{
					standard.ErrorFieldID: "main", standard.ErrorFieldField: tc.field,
				})
		}
	})

	t.Run("should report the first invalid field", func(t *testing.T) {
		_, err := create(t, container.Bag{
			standard.ConfigFieldURI:         "",
			standard.ConfigFieldMaxPoolSize: -1,
		}, nil)
		assertError(t, err, standard.ErrInvalidField, map[string]any{
			standard.ErrorFieldID: "main", standard.ErrorFieldField: standard.ConfigFieldURI,
		})
	})
}

func Test_Creator_TLS(t *testing.T) {
	t.Run("should connect with the certificates, the server name and the app name", func(t *testing.T) {
		cert := newCertificate(t)
		uri, done := tlsServer(t, cert, tls.RequireAndVerifyClientCert, []byte("flam-test-app"))
		_, err := create(t, container.Bag{
			standard.ConfigFieldURI:           uri,
			standard.ConfigFieldAppName:       "flam-test-app",
			standard.ConfigFieldTLS:           true,
			standard.ConfigFieldTLSCAFile:     cert.certFile,
			standard.ConfigFieldTLSCertFile:   cert.certFile,
			standard.ConfigFieldTLSKeyFile:    cert.keyFile,
			standard.ConfigFieldTLSServerName: "localhost",
		}, nil)
		testutil.NoError(t, err)

		h := waitHandshake(t, done)
		testutil.Equal(t, "server name", h.serverName, "localhost")
		testutil.Equal(t, "client certificates", h.peerCerts, 1)
		testutil.Equal(t, "app name sent", bytes.Contains(h.payload, []byte("flam-test-app")), true)
	})

	t.Run("should connect without checking the server when insecure", func(t *testing.T) {
		cert := newCertificate(t)
		uri, done := tlsServer(t, cert, tls.NoClientCert, nil)
		_, err := create(t, container.Bag{
			standard.ConfigFieldURI:         uri,
			standard.ConfigFieldTLS:         true,
			standard.ConfigFieldTLSInsecure: true,
		}, nil)
		testutil.NoError(t, err)

		h := waitHandshake(t, done)
		testutil.Equal(t, "client certificates", h.peerCerts, 0)
	})

	t.Run("should fail with certificate files that cannot be read", func(t *testing.T) {
		cert := newCertificate(t)
		notPEM := filepath.Join(t.TempDir(), "not.pem")
		testutil.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))
		missing := filepath.Join(t.TempDir(), "missing.pem")

		for _, tc := range []struct {
			name   string
			fields container.Bag
			field  string
		}{
			{"missing CA file", container.Bag{standard.ConfigFieldTLSCAFile: missing}, standard.ConfigFieldTLSCAFile},
			{"CA file without certificate", container.Bag{standard.ConfigFieldTLSCAFile: notPEM}, standard.ConfigFieldTLSCAFile},
			{"certificate without key", container.Bag{standard.ConfigFieldTLSCertFile: cert.certFile}, standard.ConfigFieldTLSCertFile},
			{"key without certificate", container.Bag{standard.ConfigFieldTLSKeyFile: cert.keyFile}, standard.ConfigFieldTLSCertFile},
		} {
			tc.fields[standard.ConfigFieldTLS] = true
			_, err := create(t, tc.fields, nil)
			if !strings.HasPrefix(errors.Unwrap(err).Error(), "unable to create standard mongo connection: unable to read standard mongo connection certificates: ") {
				t.Errorf("%s: Get() = %v, want the error of reading", tc.name, err)
			}
			assertError(t, err, standard.ErrTLS, map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldField: tc.field})
		}
	})
}
