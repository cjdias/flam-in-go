package app

import (
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ErrorFieldBundle", app.ErrorFieldBundle, "bundle")

	sentinels := []error{
		app.ErrNilBundle, app.ErrDuplicateBundle, app.ErrBooted,
		app.ErrInvalidFunction, app.ErrClosed,
	}
	for i, err := range sentinels {
		for j, other := range sentinels {
			if i != j && errors.Is(err, other) {
				t.Errorf("%q matches %q", err, other)
			}
		}
	}
}

func Test_Sentinels(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		text string
	}{
		{"ErrNilBundle", app.ErrNilBundle, "nil application bundle"},
		{"ErrDuplicateBundle", app.ErrDuplicateBundle, "application bundle id already registered"},
		{"ErrBooted", app.ErrBooted, "application already booted"},
		{"ErrInvalidFunction", app.ErrInvalidFunction, "invalid application function"},
		{"ErrClosed", app.ErrClosed, "application closed"},
	} {
		t.Run("should declare "+tc.name+" with its message", func(t *testing.T) {
			if tc.err == nil {
				t.Fatalf("%s is nil", tc.name)
			}
			testutil.Equal(t, tc.name+".Error()", tc.err.Error(), tc.text)
			if errors.Unwrap(tc.err) != nil {
				t.Errorf("%s wraps %v", tc.name, errors.Unwrap(tc.err))
			}
		})
	}
}
