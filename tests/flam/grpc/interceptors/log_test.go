package interceptors

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// logging returns the log interceptor "main" of the fields, with the
// clock, and the logger when not nil.
func logging(t *testing.T, driver flam.IBundle, fields container.Bag, values map[string]any, c *testutil.Clock, logger flam.ILogger) (flam.IGrpcInterceptor, error) {
	t.Helper()
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = log.Driver
	}
	providers := []any{clock(c)}
	if logger != nil {
		providers = append(providers, func() flam.ILogger { return logger })
	}
	return create(t, driver, fields, values, providers...)
}

func Test_Log_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                          {log.Id, "log"},
		"Driver":                      {log.Driver, "flam.grpc.interceptors.driver.log"},
		"CodePanic":                   {log.CodePanic, "panic"},
		"ConfigFieldChannel":          {log.ConfigFieldChannel, "channel"},
		"ConfigFieldLevel":            {log.ConfigFieldLevel, "level"},
		"ConfigFieldLevelError":       {log.ConfigFieldLevelError, "level_error"},
		"DefaultChannelConfigPath":    {log.DefaultChannelConfigPath, "flam.grpc.defaults.interceptors.log.channel"},
		"DefaultLevelConfigPath":      {log.DefaultLevelConfigPath, "flam.grpc.defaults.interceptors.log.level"},
		"DefaultLevelErrorConfigPath": {log.DefaultLevelErrorConfigPath, "flam.grpc.defaults.interceptors.log.level_error"},
		"ContextFieldMethod":          {log.ContextFieldMethod, "method"},
		"ContextFieldCode":            {log.ContextFieldCode, "code"},
		"ContextFieldDuration":        {log.ContextFieldDuration, "duration"},
		"ContextFieldPeer":            {log.ContextFieldPeer, "peer"},
		"ContextFieldError":           {log.ContextFieldError, "error"},
		"ContextFieldPanicked":        {log.ContextFieldPanicked, "panicked"},
		"ErrorFieldID":                {log.ErrorFieldID, "id"},
		"ErrorFieldField":             {log.ErrorFieldField, "field"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	testutil.Equal(t, "ErrInvalidField", log.ErrInvalidField.Error(), "invalid log grpc interceptor field")
	testutil.Equal(t, "ErrMissingLogger", log.ErrMissingLogger.Error(), "logger not provided by the container")
}

func Test_Log_Bundle(t *testing.T) {
	b := log.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.grpc.interceptors.log")

	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		flamgrpc.InterceptorConfigPath + "." + log.Id: container.Bag{factory.ConfigFieldDriver: log.Driver},
		log.DefaultChannelConfigPath:                  "flam.grpc",
		log.DefaultLevelConfigPath:                    "info",
		log.DefaultLevelErrorConfigPath:               "error",
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}
}

func Test_Log_Interceptor(t *testing.T) {
	notFound := status.Error(codes.NotFound, "no user")
	internal := status.Error(codes.Internal, "broken")
	for name, tc := range map[string]struct {
		err   error
		level flam.LogLevel
		code  string
	}{
		"a successful call":          {nil, flam.LogInfo, "OK"},
		"a call failing":             {notFound, flam.LogInfo, "NotFound"},
		"a call failing the server":  {internal, flam.LogError, "Internal"},
		"a call of a non status err": {errBoom, flam.LogError, "Unknown"},
	} {
		t.Run("should signal "+name, func(t *testing.T) {
			logger, c := &entries{}, newClock()
			i, err := logging(t, log.NewBundle(), container.Bag{}, nil, c, logger)
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", i.Id(), "main")

			resp, err := unary(i, withPeer(context.Background(), "10.0.0.1"), c, "resp", tc.err, false)
			testutil.Equal(t, "resp", resp, "resp")
			testutil.Equal(t, "unary error", err, tc.err)
			testutil.Equal(t, "stream error", streaming(i, context.Background(), c, tc.err, false), tc.err)

			want := container.Bag{
				log.ContextFieldMethod:   fullMethod,
				log.ContextFieldCode:     tc.code,
				log.ContextFieldDuration: time.Second,
			}
			if tc.err != nil {
				want[log.ContextFieldError] = tc.err
			}
			unaryWant := container.Bag{log.ContextFieldPeer: "10.0.0.1:5000"}
			for field, value := range want {
				unaryWant[field] = value
			}
			testutil.Equal(t, "entries", logger.recorded(), []entry{
				{tc.level, "flam.grpc", "grpc call served", unaryWant},
				{tc.level, "flam.grpc", "grpc call served", want},
			})
		})
	}

	t.Run("should signal the calls whose handler panicked, the panic going on", func(t *testing.T) {
		logger, c := &entries{}, newClock()
		i, err := logging(t, log.NewBundle(), container.Bag{}, nil, c, logger)
		testutil.NoError(t, err)

		testutil.Panics(t, "unary", func() { _, _ = unary(i, context.Background(), c, nil, nil, true) })
		testutil.Panics(t, "stream", func() { _ = streaming(i, context.Background(), c, nil, true) })
		want := container.Bag{
			log.ContextFieldMethod:   fullMethod,
			log.ContextFieldCode:     log.CodePanic,
			log.ContextFieldDuration: time.Second,
			log.ContextFieldPanicked: true,
		}
		testutil.Equal(t, "entries", logger.recorded(), []entry{
			{flam.LogError, "flam.grpc", "grpc call served", want},
			{flam.LogError, "flam.grpc", "grpc call served", want},
		})
	})

	t.Run("should leave out a peer of no address", func(t *testing.T) {
		logger, c := &entries{}, newClock()
		i, err := logging(t, log.NewBundle(), container.Bag{}, nil, c, logger)
		testutil.NoError(t, err)
		_, _ = unary(i, peer.NewContext(context.Background(), &peer.Peer{}), nil, nil, nil, false)
		_, has := logger.recorded()[0].ctx[log.ContextFieldPeer]
		testutil.Equal(t, "peer", has, false)
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		logger, c := &entries{}, newClock()
		i, err := logging(t, log.NewBundle(), container.Bag{
			log.ConfigFieldChannel: "rpc",
			log.ConfigFieldLevel:   "debug",
		}, map[string]any{log.DefaultLevelErrorConfigPath: "warning"}, c, logger)
		testutil.NoError(t, err)
		_, _ = unary(i, context.Background(), nil, nil, nil, false)
		_, _ = unary(i, context.Background(), nil, nil, errBoom, false)
		signaled := logger.recorded()
		testutil.Equal(t, "levels", []flam.LogLevel{signaled[0].level, signaled[1].level}, []flam.LogLevel{flam.LogDebug, flam.LogWarning})
		testutil.Equal(t, "channel", signaled[0].channel, "rpc")
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		logger, c := &entries{}, newClock()
		i, err := logging(t, bare{log.NewBundle()}, container.Bag{}, nil, c, logger)
		testutil.NoError(t, err)
		_, _ = unary(i, context.Background(), nil, nil, nil, false)
		_, _ = unary(i, context.Background(), nil, nil, errBoom, false)
		signaled := logger.recorded()
		testutil.Equal(t, "levels", []flam.LogLevel{signaled[0].level, signaled[1].level}, []flam.LogLevel{flam.LogInfo, flam.LogError})
		testutil.Equal(t, "channel", signaled[0].channel, "flam.grpc")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			log.ConfigFieldChannel:    1,
			log.ConfigFieldLevel:      "loud",
			log.ConfigFieldLevelError: 99,
		} {
			_, err := logging(t, log.NewBundle(), container.Bag{field: val}, nil, newClock(), &entries{})
			testutil.Error(t, errors.Unwrap(err), log.ErrInvalidField,
				"unable to create log grpc interceptor: invalid log grpc interceptor field",
				map[string]any{log.ErrorFieldID: "main", log.ErrorFieldField: field})
		}
	})

	t.Run("should fail without logger in the container", func(t *testing.T) {
		_, err := logging(t, log.NewBundle(), container.Bag{}, nil, newClock(), nil)
		testutil.Error(t, errors.Unwrap(err), log.ErrMissingLogger,
			"unable to create log grpc interceptor: logger not provided by the container",
			map[string]any{log.ErrorFieldID: "main"})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := logging(t, log.NewBundle(), container.Bag{factory.ConfigFieldDriver: "other"}, nil, newClock(), &entries{})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
