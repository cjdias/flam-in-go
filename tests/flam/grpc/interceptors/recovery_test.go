package interceptors

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors/recovery"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// recovering returns the recovery interceptor "main" of the fields, with
// the logger when not nil.
func recovering(t *testing.T, driver flam.IBundle, fields container.Bag, values map[string]any, logger flam.ILogger) (flam.IGrpcInterceptor, error) {
	t.Helper()
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = recovery.Driver
	}
	var providers []any
	if logger != nil {
		providers = append(providers, func() flam.ILogger { return logger })
	}
	return create(t, driver, fields, values, providers...)
}

// panicEntry checks the entry of a panic of the call.
func panicEntry(t *testing.T, e entry, level flam.LogLevel, channel string) {
	t.Helper()
	testutil.Equal(t, "level", e.level, level)
	testutil.Equal(t, "channel", e.channel, channel)
	testutil.Equal(t, "message", e.message, "grpc handler panicked")
	testutil.Equal(t, "method", e.ctx[recovery.ContextFieldMethod], fullMethod)
	testutil.Equal(t, "panic", e.ctx[recovery.ContextFieldPanic], "boom")
	if stack, _ := e.ctx[recovery.ContextFieldStack].(string); !strings.Contains(stack, "goroutine") {
		t.Errorf("stack = %q, want the stack of the handler", stack)
	}
	testutil.Equal(t, "fields", len(e.ctx), 3)
}

func Test_Recovery_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                       {recovery.Id, "recovery"},
		"Driver":                   {recovery.Driver, "flam.grpc.interceptors.driver.recovery"},
		"Message":                  {recovery.Message, "internal error"},
		"ConfigFieldChannel":       {recovery.ConfigFieldChannel, "channel"},
		"ConfigFieldLevel":         {recovery.ConfigFieldLevel, "level"},
		"DefaultChannelConfigPath": {recovery.DefaultChannelConfigPath, "flam.grpc.defaults.interceptors.recovery.channel"},
		"DefaultLevelConfigPath":   {recovery.DefaultLevelConfigPath, "flam.grpc.defaults.interceptors.recovery.level"},
		"ContextFieldMethod":       {recovery.ContextFieldMethod, "method"},
		"ContextFieldPanic":        {recovery.ContextFieldPanic, "panic"},
		"ContextFieldStack":        {recovery.ContextFieldStack, "stack"},
		"ErrorFieldID":             {recovery.ErrorFieldID, "id"},
		"ErrorFieldField":          {recovery.ErrorFieldField, "field"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	testutil.Equal(t, "ErrInvalidField", recovery.ErrInvalidField.Error(), "invalid recovery grpc interceptor field")
	testutil.Equal(t, "ErrMissingLogger", recovery.ErrMissingLogger.Error(), "logger not provided by the container")
}

func Test_Recovery_Bundle(t *testing.T) {
	b := recovery.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.grpc.interceptors.recovery")

	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		flamgrpc.InterceptorConfigPath + "." + recovery.Id: container.Bag{factory.ConfigFieldDriver: recovery.Driver},
		recovery.DefaultChannelConfigPath:                  "flam.grpc",
		recovery.DefaultLevelConfigPath:                    "error",
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}
}

func Test_Recovery_Interceptor(t *testing.T) {
	t.Run("should answer a panic of a unary call with an internal status", func(t *testing.T) {
		logger := &entries{}
		i, err := recovering(t, recovery.NewBundle(), container.Bag{}, nil, logger)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", i.Id(), "main")

		resp, err := unary(i, context.Background(), nil, "resp", nil, true)
		testutil.Equal(t, "resp", resp, nil)
		testutil.Equal(t, "code", status.Code(err), codes.Internal)
		testutil.Equal(t, "message", status.Convert(err).Message(), "internal error")
		signaled := logger.recorded()
		testutil.Equal(t, "entries", len(signaled), 1)
		panicEntry(t, signaled[0], flam.LogError, "flam.grpc")
	})

	t.Run("should answer a panic of a streaming call with an internal status", func(t *testing.T) {
		logger := &entries{}
		i, err := recovering(t, recovery.NewBundle(), container.Bag{}, nil, logger)
		testutil.NoError(t, err)

		err = streaming(i, context.Background(), nil, nil, true)
		testutil.Equal(t, "code", status.Code(err), codes.Internal)
		testutil.Equal(t, "message", status.Convert(err).Message(), "internal error")
		signaled := logger.recorded()
		testutil.Equal(t, "entries", len(signaled), 1)
		panicEntry(t, signaled[0], flam.LogError, "flam.grpc")
	})

	t.Run("should leave the calls not panicking alone", func(t *testing.T) {
		logger := &entries{}
		i, err := recovering(t, recovery.NewBundle(), container.Bag{}, nil, logger)
		testutil.NoError(t, err)

		resp, err := unary(i, context.Background(), nil, "resp", errBoom, false)
		testutil.Equal(t, "resp", resp, "resp")
		testutil.Equal(t, "unary error", err, errBoom)
		testutil.Equal(t, "stream error", streaming(i, context.Background(), nil, errBoom, false), errBoom)
		testutil.NoError(t, streaming(i, context.Background(), nil, nil, false))
		testutil.Equal(t, "entries", len(logger.recorded()), 0)
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		logger := &entries{}
		i, err := recovering(t, recovery.NewBundle(), container.Bag{recovery.ConfigFieldChannel: "rpc"},
			map[string]any{recovery.DefaultLevelConfigPath: "warning"}, logger)
		testutil.NoError(t, err)
		_, _ = unary(i, context.Background(), nil, nil, nil, true)
		panicEntry(t, logger.recorded()[0], flam.LogWarning, "rpc")
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		logger := &entries{}
		i, err := recovering(t, bare{recovery.NewBundle()}, container.Bag{}, nil, logger)
		testutil.NoError(t, err)
		_, _ = unary(i, context.Background(), nil, nil, nil, true)
		panicEntry(t, logger.recorded()[0], flam.LogError, "flam.grpc")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			recovery.ConfigFieldChannel: "",
			recovery.ConfigFieldLevel:   "loud",
		} {
			_, err := recovering(t, recovery.NewBundle(), container.Bag{field: val}, nil, &entries{})
			testutil.Error(t, errors.Unwrap(err), recovery.ErrInvalidField,
				"unable to create recovery grpc interceptor: invalid recovery grpc interceptor field",
				map[string]any{recovery.ErrorFieldID: "main", recovery.ErrorFieldField: field})
		}
	})

	t.Run("should fail without logger in the container", func(t *testing.T) {
		_, err := recovering(t, recovery.NewBundle(), container.Bag{}, nil, nil)
		testutil.Error(t, errors.Unwrap(err), recovery.ErrMissingLogger,
			"unable to create recovery grpc interceptor: logger not provided by the container",
			map[string]any{recovery.ErrorFieldID: "main"})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := recovering(t, recovery.NewBundle(), container.Bag{factory.ConfigFieldDriver: "other"}, nil, &entries{})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
