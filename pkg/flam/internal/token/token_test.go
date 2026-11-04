package token

import (
	"encoding/hex"
	"testing"
)

// The test lives next to the package, unlike the other tests of flam,
// since an internal package cannot be imported from the tests directory.

func Test_New(t *testing.T) {
	first, second := New(), New()
	for _, got := range []string{first, second} {
		if len(got) != 32 {
			t.Errorf("len(New()) = %d, want 32", len(got))
		}
		if _, err := hex.DecodeString(got); err != nil {
			t.Errorf("New() = %q, want hexadecimal digits: %v", got, err)
		}
	}
	if first == second {
		t.Errorf("New() = %q twice, want distinct tokens", first)
	}
}
