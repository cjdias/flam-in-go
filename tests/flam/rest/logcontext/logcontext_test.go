package logcontext

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/logcontext"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// extractor returns the extractor provided by the bundle to an
// application whose configuration holds the defaults.
func extractor(t *testing.T, defaults container.Bag) flam.ILogContextExtractor {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, app.RegisterBundle(logcontext.NewBundle()))
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(defaults) }))
	var list []flam.ILogContextExtractor
	testutil.NoError(t, app.Invoke(func(in struct {
		dig.In
		Extractors []flam.ILogContextExtractor `group:"flam.log.context.extractors"`
	}) {
		list = in.Extractors
	}))
	testutil.Equal(t, "extractors", len(list), 1)
	return list[0]
}

// requestContext returns a context holding the request id and the
// principal, each left out when empty.
func requestContext(id, principal string) context.Context {
	ctx := context.Background()
	if id != "" {
		ctx = rest.WithRequestId(ctx, id)
	}
	if principal != "" {
		ctx = rest.WithPrincipal(ctx, principal)
	}
	return ctx
}

func Test_Bundle(t *testing.T) {
	t.Run("should have its id, paths and fields", func(t *testing.T) {
		testutil.Equal(t, "Id()", logcontext.NewBundle().Id(), "flam.rest.logcontext")
		testutil.Equal(t, "request id path", logcontext.RequestIdFieldConfigPath, "flam.rest.logcontext.request_id")
		testutil.Equal(t, "principal path", logcontext.PrincipalFieldConfigPath, "flam.rest.logcontext.principal")
		testutil.Equal(t, "request id", logcontext.ContextFieldRequestId, "request_id")
		testutil.Equal(t, "principal", logcontext.ContextFieldPrincipal, "principal")
	})

	t.Run("should register its provider in the extractor group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, logcontext.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		boom := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return boom }
		if err := logcontext.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

func Test_Extractor(t *testing.T) {
	t.Run("should extract the request id and the principal", func(t *testing.T) {
		got := extractor(t, container.Bag{}).Extract(requestContext("req-1", "alice"))
		testutil.Equal(t, "fields", got, flam.IBag(container.Bag{"request_id": "req-1", "principal": "alice"}))
	})

	t.Run("should leave out what the context does not hold", func(t *testing.T) {
		e := extractor(t, container.Bag{})
		testutil.Equal(t, "id only", e.Extract(requestContext("req-1", "")), flam.IBag(container.Bag{"request_id": "req-1"}))
		testutil.Equal(t, "principal only", e.Extract(requestContext("", "alice")), flam.IBag(container.Bag{"principal": "alice"}))
		testutil.Equal(t, "none", e.Extract(context.Background()), flam.IBag(container.Bag{}))
		//lint:ignore SA1012 a nil context is what is tested
		testutil.Equal(t, "nil", e.Extract(nil), flam.IBag(container.Bag{})) //nolint:staticcheck // as above
	})

	t.Run("should extract at the configured fields, leaving out the empty ones", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(logcontext.RequestIdFieldConfigPath, "rid"))
		testutil.NoError(t, bag.Set(logcontext.PrincipalFieldConfigPath, ""))
		got := extractor(t, bag).Extract(requestContext("req-1", "alice"))
		testutil.Equal(t, "fields", got, flam.IBag(container.Bag{"rid": "req-1"}))

		bag = container.Bag{}
		testutil.NoError(t, bag.Set(logcontext.RequestIdFieldConfigPath, ""))
		testutil.NoError(t, bag.Set(logcontext.PrincipalFieldConfigPath, "who"))
		got = extractor(t, bag).Extract(requestContext("req-1", "alice"))
		testutil.Equal(t, "fields", got, flam.IBag(container.Bag{"who": "alice"}))
	})
}
