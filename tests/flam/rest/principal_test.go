package rest

import (
	"context"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Principal(t *testing.T) {
	t.Run("should hold no principal in a nil or bare context", func(t *testing.T) {
		var nilCtx context.Context
		principal, ok := rest.PrincipalOf(nilCtx)
		testutil.Equal(t, "nil principal", principal, "")
		testutil.Equal(t, "nil ok", ok, false)

		principal, ok = rest.PrincipalOf(context.Background())
		testutil.Equal(t, "bare principal", principal, "")
		testutil.Equal(t, "bare ok", ok, false)
	})

	t.Run("should return the principal set", func(t *testing.T) {
		ctx := rest.WithPrincipal(context.Background(), "alice")
		principal, ok := rest.PrincipalOf(ctx)
		testutil.Equal(t, "principal", principal, "alice")
		testutil.Equal(t, "ok", ok, true)

		principal, ok = rest.PrincipalOf(rest.WithPrincipal(ctx, "bob"))
		testutil.Equal(t, "replaced principal", principal, "bob")
		testutil.Equal(t, "replaced ok", ok, true)
	})

	t.Run("should take an empty principal for none", func(t *testing.T) {
		ctx := rest.WithPrincipal(rest.WithPrincipal(context.Background(), "alice"), "")
		principal, ok := rest.PrincipalOf(ctx)
		testutil.Equal(t, "principal", principal, "")
		testutil.Equal(t, "ok", ok, false)
	})
}
