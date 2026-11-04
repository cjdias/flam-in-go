package settings

import (
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// The test lives next to the package, unlike the other tests of flam,
// since an internal package cannot be imported from the tests directory.

// config is a configuration of the values of its bag.
type config struct {
	flam.IConfig

	values container.Bag
}

func (c config) Has(path string) bool { return c.values.Has(path) }

func (c config) Get(path string, def ...any) any { return c.values.Get(path, def...) }

// equal fails the test when got is not want.
func equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func Test_Field(t *testing.T) {
	cfg := config{values: container.Bag{"defaults": container.Bag{"a": "default"}}}
	for name, tc := range map[string]struct {
		bag         container.Bag
		defaultPath string
		want        any
	}{
		"field":                  {container.Bag{"a": "field"}, "defaults.a", "field"},
		"nil field":              {container.Bag{"a": nil}, "defaults.a", nil},
		"default":                {container.Bag{}, "defaults.a", "default"},
		"built-in":               {container.Bag{}, "defaults.b", "built-in"},
		"no default path":        {container.Bag{}, "", "built-in"},
		"field, no default path": {container.Bag{"a": 1}, "", 1},
	} {
		equal(t, name, Field(cfg, tc.bag, "a", tc.defaultPath, "built-in"), tc.want)
	}
}

func Test_Duration(t *testing.T) {
	for name, tc := range map[string]struct {
		value any
		want  time.Duration
		ok    bool
	}{
		"duration":          {time.Second, time.Second, true},
		"zero":              {time.Duration(0), 0, true},
		"text":              {"1m", time.Minute, true},
		"negative duration": {-time.Second, -time.Second, false},
		"negative text":     {"-1s", -time.Second, false},
		"invalid text":      {"soon", -1, false},
		"number":            {1, -1, false},
		"nil":               {nil, -1, false},
	} {
		d, ok := Duration(tc.value)
		equal(t, name+" duration", d, tc.want)
		equal(t, name+" ok", ok, tc.ok)
	}
}

func Test_Strings(t *testing.T) {
	for name, tc := range map[string]struct {
		value any
		want  []string
		ok    bool
	}{
		"nil":            {nil, nil, true},
		"strings":        {[]string{"a", "b"}, []string{"a", "b"}, true},
		"any":            {[]any{"a", "b"}, []string{"a", "b"}, true},
		"empty any":      {[]any{}, []string{}, true},
		"any non string": {[]any{"a", 1}, nil, false},
		"string":         {"a", nil, false},
		"ints":           {[]int{1}, nil, false},
	} {
		got, ok := Strings(tc.value)
		equal(t, name+" ok", ok, tc.ok)
		equal(t, name+" len", len(got), len(tc.want))
		equal(t, name+" nil", got == nil, tc.want == nil)
		for i := range tc.want {
			equal(t, name+" item", got[i], tc.want[i])
		}
	}
}
