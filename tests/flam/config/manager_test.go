package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fakeSource is a flam.IConfigSource holding a bag.
type fakeSource struct {
	id       string
	priority int

	mu  sync.Mutex
	bag flam.IBag
}

func (s *fakeSource) Id() string    { return s.id }
func (s *fakeSource) Close() error  { return nil }
func (s *fakeSource) Priority() int { return s.priority }

func (s *fakeSource) Bag() flam.IBag {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bag == nil {
		return nil
	}
	return s.bag.Clone()
}

// reloadSource is a flam.IConfigReloadableSource whose Reload calls a
// function, which may change its bag.
type reloadSource struct {
	*fakeSource
	reload func(s *reloadSource) (bool, error)
}

func (s *reloadSource) Reload() (bool, error) { return s.reload(s) }

// set changes the bag of the source.
func (s *fakeSource) set(bag flam.IBag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bag = bag
}

// source returns a fake source.
func source(id string, priority int, bag container.Bag) *fakeSource {
	return &fakeSource{id: id, priority: priority, bag: bag}
}

// reloadable returns a reloadable source changing to the next bags, one
// per Reload, reporting a change, then reporting none.
func reloadable(id string, priority int, bag container.Bag, next ...container.Bag) *reloadSource {
	return &reloadSource{fakeSource: source(id, priority, bag), reload: func(s *reloadSource) (bool, error) {
		if len(next) == 0 {
			return false, nil
		}
		s.set(next[0])
		next = next[1:]
		return true, nil
	}}
}

// emptyConfig returns the configuration provided by the config bundle,
// registered in a new testutil.App.
func emptyConfig() flam.IConfig {
	app := testutil.NewApp()
	if err := config.NewBundle().Register(app); err != nil {
		panic(err)
	}
	var cfg flam.IConfig
	if err := app.Invoke(func(c flam.IConfig) { cfg = c }); err != nil {
		panic(err)
	}
	return cfg
}

// newConfig returns a configuration with the defaults and the sources.
func newConfig(t *testing.T, defaults container.Bag, sources ...flam.IConfigSource) flam.IConfig {
	t.Helper()
	c := emptyConfig()
	if defaults != nil {
		testutil.NoError(t, c.AddDefaults(defaults))
	}
	for _, s := range sources {
		testutil.NoError(t, c.AddSource(s))
	}
	return c
}

// change is a call of an observer.
type change struct {
	id            string
	old, newValue any
}

// recorder records the calls of its observers.
type recorder struct {
	mu    sync.Mutex
	calls []change
}

// observe adds an observer recording its calls.
func (r *recorder) observe(t *testing.T, c flam.IConfig, id, path string) {
	t.Helper()
	testutil.NoError(t, c.AddObserver(id, path, func(old, newValue any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.calls = append(r.calls, change{id, old, newValue})
	}))
}

// take returns and forgets the calls recorded.
func (r *recorder) take() []change {
	r.mu.Lock()
	defer r.mu.Unlock()
	calls := r.calls
	r.calls = nil
	return calls
}

// failingBag is a flam.IBag that cannot be merged, its Get("") returning no
// bag.
type failingBag struct {
	flam.IBag
}

func (failingBag) Get(string, ...any) any { return 1 }
func (failingBag) Clone() flam.IBag       { return failingBag{} }

// ---------------------------------------------------------------------------
// creation and defaults
// ---------------------------------------------------------------------------

func Test_New(t *testing.T) {
	t.Run("should hold nothing", func(t *testing.T) {
		c := emptyConfig()
		testutil.Equal(t, "Entries()", len(c.Entries()), 0)
		testutil.Equal(t, "Has()", c.Has(""), true)
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{}))
		testutil.Equal(t, "HasSource()", c.HasSource("a"), false)
		testutil.Equal(t, "HasObserver()", c.HasObserver("a", ""), false)
	})

	t.Run("should return a new configuration per call", func(t *testing.T) {
		a, b := emptyConfig(), emptyConfig()
		testutil.NoError(t, a.Set("a", 1))
		testutil.Equal(t, "other", b.Has("a"), false)
	})
}

func Test_AddDefaults(t *testing.T) {
	t.Run("should add the defaults, the first added winning", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1, "b": container.Bag{"c": 1}})
		testutil.NoError(t, c.AddDefaults(container.Bag{"a": 2, "b": container.Bag{"c": 2, "d": 2}, "e": 2}))
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{
			"a": 1, "b": container.Bag{"c": 1, "d": 2}, "e": 2,
		}))
	})

	t.Run("should expand the dotted keys", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a.b": 1})
		testutil.Equal(t, "a.b", c.Int("a.b"), 1)
	})

	t.Run("should copy the defaults", func(t *testing.T) {
		defaults := container.Bag{"a": container.Bag{"b": 1}, "l": []any{1}}
		c := newConfig(t, defaults)
		testutil.NoError(t, defaults.Set("a.b", 2))
		defaults["l"].([]any)[0] = 2
		testutil.Equal(t, "a.b", c.Int("a.b"), 1)
		testutil.Equal(t, "l", c.Slice("l"), []any{1})
	})

	t.Run("should copy the maps and slices of any type", func(t *testing.T) {
		ints, strs := []int{1, 2}, map[string]string{"k": "v"}
		nested := []any{map[any]any{"x": 1}}
		c := newConfig(t, container.Bag{"ints": ints, "strs": strs, "nested": nested})
		ints[0], strs["k"] = 99, "changed"
		nested[0].(map[any]any)["x"] = 2
		testutil.Equal(t, "ints", c.Get("ints"), any([]int{1, 2}))
		testutil.Equal(t, "strs", c.Get("strs"), any(map[string]string{"k": "v"}))
		testutil.Equal(t, "nested", c.Get("nested"), any([]any{map[any]any{"x": 1}}))
	})

	t.Run("should accept nil defaults", func(t *testing.T) {
		c := emptyConfig()
		testutil.NoError(t, c.AddDefaults(nil))
		testutil.Equal(t, "Entries()", len(c.Entries()), 0)
	})

	t.Run("should be overridden by the sources and Set", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1, "b": 1, "c": 1}, source("s", 0, container.Bag{"b": 2, "c": 2}))
		testutil.NoError(t, c.Set("c", 3))
		testutil.NoError(t, c.AddDefaults(container.Bag{"d": 4}))
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 1, "b": 2, "c": 3, "d": 4}))
	})

	t.Run("should refuse defaults holding an invalid key, unchanged", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1})
		err := c.AddDefaults(container.Bag{"b": 1, "c..d": 1})
		testutil.Error(t, err, container.ErrInvalidPath, "unable to merge bag: invalid path",
			map[string]any{"path": "", "key": "c..d"})
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 1}))
	})

	t.Run("should call the observers of the values changed", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1})
		var r recorder
		r.observe(t, c, "a", "a")
		r.observe(t, c, "b", "b")
		testutil.NoError(t, c.AddDefaults(container.Bag{"a": 2, "b": 2}))
		testutil.Equal(t, "calls", r.take(), []change{{"b", nil, 2}})
	})
}

// ---------------------------------------------------------------------------
// getters
// ---------------------------------------------------------------------------

func Test_Getters(t *testing.T) {
	c := newConfig(t, container.Bag{
		"bool": true, "int": 1, "int8": int8(2), "int16": int16(3), "int32": int32(4), "int64": int64(5),
		"uint": uint(6), "uint8": uint8(7), "uint16": uint16(8), "uint32": uint32(9), "uint64": uint64(10),
		"float32": float32(1.5), "float64": 2.5, "rune": 'r', "string": "s",
		"bag": container.Bag{"a": 1}, "nested": container.Bag{"b": container.Bag{"c": "d"}},
	})

	t.Run("should list the top level keys, sorted", func(t *testing.T) {
		testutil.Equal(t, "Entries()", c.Entries(), []string{
			"bag", "bool", "float32", "float64", "int", "int16", "int32", "int64", "int8", "nested",
			"rune", "string", "uint", "uint16", "uint32", "uint64", "uint8",
		})
	})

	t.Run("should report the paths holding a value", func(t *testing.T) {
		for path, want := range map[string]bool{
			"int": true, "nested.b.c": true, "nested.b": true, "": true,
			"missing": false, "int.a": false, "nested..b": false,
		} {
			testutil.Equal(t, path, c.Has(path), want)
		}
	})

	t.Run("should read the scalars as the bag does", func(t *testing.T) {
		testutil.Equal(t, "Bool", c.Bool("bool"), true)
		testutil.Equal(t, "Int", c.Int("int"), 1)
		testutil.Equal(t, "Int8", c.Int8("int8"), int8(2))
		testutil.Equal(t, "Int16", c.Int16("int16"), int16(3))
		testutil.Equal(t, "Int32", c.Int32("int32"), int32(4))
		testutil.Equal(t, "Int64", c.Int64("int64"), int64(5))
		testutil.Equal(t, "Uint", c.Uint("uint"), uint(6))
		testutil.Equal(t, "Uint8", c.Uint8("uint8"), uint8(7))
		testutil.Equal(t, "Uint16", c.Uint16("uint16"), uint16(8))
		testutil.Equal(t, "Uint32", c.Uint32("uint32"), uint32(9))
		testutil.Equal(t, "Uint64", c.Uint64("uint64"), uint64(10))
		testutil.Equal(t, "Float32", c.Float32("float32"), float32(1.5))
		testutil.Equal(t, "Float64", c.Float64("float64"), 2.5)
		testutil.Equal(t, "Rune", c.Rune("rune"), 'r')
		testutil.Equal(t, "String", c.String("string"), "s")
		testutil.Equal(t, "nested String", c.String("nested.b.c"), "d")
	})

	t.Run("should return the default or the zero value of a missing or other value", func(t *testing.T) {
		testutil.Equal(t, "Bool", c.Bool("missing", true), true)
		testutil.Equal(t, "Int", c.Int("string", 7), 7)
		testutil.Equal(t, "Int8", c.Int8("missing", 7), int8(7))
		testutil.Equal(t, "Int16", c.Int16("missing", 7), int16(7))
		testutil.Equal(t, "Int32", c.Int32("missing", 7), int32(7))
		testutil.Equal(t, "Int64", c.Int64("int", 7), int64(7))
		testutil.Equal(t, "Uint", c.Uint("missing", 7), uint(7))
		testutil.Equal(t, "Uint8", c.Uint8("missing", 7), uint8(7))
		testutil.Equal(t, "Uint16", c.Uint16("missing", 7), uint16(7))
		testutil.Equal(t, "Uint32", c.Uint32("missing", 7), uint32(7))
		testutil.Equal(t, "Uint64", c.Uint64("missing", 7), uint64(7))
		testutil.Equal(t, "Float32", c.Float32("missing", 7), float32(7))
		testutil.Equal(t, "Float64", c.Float64("int", 7), 7.0)
		testutil.Equal(t, "Rune", c.Rune("missing", 'x'), 'x')
		testutil.Equal(t, "String", c.String("int", "x"), "x")
		testutil.Equal(t, "String zero", c.String("missing"), "")
		testutil.Equal(t, "Int zero", c.Int("missing"), 0)
	})

	t.Run("should return a copy of the value, or the default", func(t *testing.T) {
		got := c.Get("nested").(container.Bag)
		testutil.NoError(t, got.Set("b.c", "changed"))
		testutil.Equal(t, "nested.b.c", c.String("nested.b.c"), "d")
		testutil.Equal(t, "default", c.Get("missing", "x"), any("x"))
		testutil.Equal(t, "no default", c.Get("missing"), nil)
		testutil.Equal(t, "scalar", c.Get("int"), any(1))
	})
}

func Test_Getters_Collections(t *testing.T) {
	c := newConfig(t, container.Bag{
		"bag":     container.Bag{"a": 1, "b": container.Bag{"c": 2}},
		"map":     map[string]any{"a": "x"},
		"nilBag":  container.Bag(nil),
		"strings": container.Bag{"a": "x", "b": "y"},
		"mixed":   container.Bag{"a": "x", "b": 1},
		"list":    []any{1, container.Bag{"a": 1}},
		"names":   []string{"a", "b"},
		"anyList": []any{"a", "b"},
		"scalar":  1,
	})
	testutil.NoError(t, c.Set("smap", map[string]string{"a": "x"}))

	t.Run("should return a copy of a bag", func(t *testing.T) {
		got := c.Bag("bag")
		testutil.Equal(t, "Bag()", got, flam.IBag(container.Bag{"a": 1, "b": container.Bag{"c": 2}}))
		testutil.NoError(t, got.Set("b.c", 3))
		testutil.Equal(t, "bag.b.c", c.Int("bag.b.c"), 2)
		testutil.Equal(t, "map", c.Bag("map"), flam.IBag(container.Bag{"a": "x"}))
		testutil.Equal(t, "whole", c.Bag("").Int("bag.a"), 1)
		testutil.Equal(t, "nil bag", c.Bag("nilBag", container.Bag{}), nil)
		testutil.Equal(t, "default", c.Bag("scalar", container.Bag{"d": 1}), flam.IBag(container.Bag{"d": 1}))
		testutil.Equal(t, "missing", c.Bag("missing"), nil)
	})

	t.Run("should return a copy of a bag as a map", func(t *testing.T) {
		got := c.StringMap("bag")
		testutil.Equal(t, "StringMap()", got, map[string]any{"a": 1, "b": container.Bag{"c": 2}})
		got["a"] = 5
		got["b"].(container.Bag)["c"] = 5
		testutil.Equal(t, "bag", c.Get("bag"), any(container.Bag{"a": 1, "b": container.Bag{"c": 2}}))
		testutil.Equal(t, "map", c.StringMap("map"), map[string]any{"a": "x"})
		testutil.Equal(t, "nil bag", c.StringMap("nilBag", map[string]any{}), map[string]any(nil))
		testutil.Equal(t, "default", c.StringMap("scalar", map[string]any{"d": 1}), map[string]any{"d": 1})
		testutil.Equal(t, "missing", c.StringMap("missing"), map[string]any(nil))
	})

	t.Run("should return a bag of strings as a new map of strings", func(t *testing.T) {
		got := c.StringMapString("strings")
		testutil.Equal(t, "StringMapString()", got, map[string]string{"a": "x", "b": "y"})
		got["a"] = "changed"
		testutil.Equal(t, "strings.a", c.String("strings.a"), "x")
		testutil.Equal(t, "map", c.StringMapString("map"), map[string]string{"a": "x"})
		testutil.Equal(t, "nil bag", c.StringMapString("nilBag"), map[string]string{})
		testutil.Equal(t, "mixed", c.StringMapString("mixed", map[string]string{"d": "e"}),
			map[string]string{"d": "e"})
		testutil.Equal(t, "nested", c.StringMapString("bag"), map[string]string(nil))
		testutil.Equal(t, "scalar", c.StringMapString("scalar"), map[string]string(nil))
		testutil.Equal(t, "missing", c.StringMapString("missing", map[string]string{}), map[string]string{})
	})

	t.Run("should return a copy of a stored map of strings", func(t *testing.T) {
		got := c.StringMapString("smap")
		testutil.Equal(t, "StringMapString()", got, map[string]string{"a": "x"})
		got["a"] = "changed"
		testutil.Equal(t, "again", c.StringMapString("smap"), map[string]string{"a": "x"})
	})

	t.Run("should return a copy of a list", func(t *testing.T) {
		got := c.Slice("list")
		testutil.Equal(t, "Slice()", got, []any{1, container.Bag{"a": 1}})
		got[0] = 2
		got[1].(container.Bag)["a"] = 2
		testutil.Equal(t, "list", c.Slice("list"), []any{1, container.Bag{"a": 1}})
		testutil.Equal(t, "names", c.Slice("names"), []any{"a", "b"})
		testutil.Equal(t, "default", c.Slice("scalar", []any{0}), []any{0})
		testutil.Equal(t, "missing", c.Slice("missing"), []any(nil))
	})

	t.Run("should return a copy of a list of strings", func(t *testing.T) {
		got := c.StringSlice("names")
		testutil.Equal(t, "StringSlice()", got, []string{"a", "b"})
		got[0] = "changed"
		testutil.Equal(t, "names", c.StringSlice("names"), []string{"a", "b"})
		testutil.Equal(t, "anyList", c.StringSlice("anyList"), []string{"a", "b"})
		testutil.Equal(t, "default", c.StringSlice("list", []string{"d"}), []string{"d"})
		testutil.Equal(t, "missing", c.StringSlice("missing"), []string(nil))
	})
}

func Test_Getters_Duration(t *testing.T) {
	c := newConfig(t, container.Bag{"duration": 5 * time.Second, "text": "1m30s", "bad": "soon", "int": 5})
	testutil.Equal(t, "duration", c.Duration("duration"), 5*time.Second)
	testutil.Equal(t, "text", c.Duration("text"), 90*time.Second)
	testutil.Equal(t, "bad", c.Duration("bad", time.Hour), time.Hour)
	testutil.Equal(t, "int", c.Duration("int", time.Hour), time.Hour)
	testutil.Equal(t, "missing", c.Duration("missing"), time.Duration(0))
}

// ---------------------------------------------------------------------------
// Set
// ---------------------------------------------------------------------------

func Test_Set(t *testing.T) {
	t.Run("should store a copy of the value", func(t *testing.T) {
		c := emptyConfig()
		value := container.Bag{"b": []any{1}}
		testutil.NoError(t, c.Set("a", value))
		value["b"].([]any)[0] = 2
		testutil.Equal(t, "a", c.Get("a"), any(container.Bag{"b": []any{1}}))
	})

	t.Run("should store and return copies of maps and slices of any type", func(t *testing.T) {
		c := emptyConfig()
		strs := map[string]string{"a": "x"}
		ints := []int{1}
		nested := map[int][]any{1: {map[string]int{"a": 1}, nil}}
		testutil.NoError(t, c.Set("s", strs))
		testutil.NoError(t, c.Set("i", ints))
		testutil.NoError(t, c.Set("n", nested))
		testutil.NoError(t, c.Set("nil", map[string]int(nil)))
		testutil.NoError(t, c.Set("nils", []int(nil)))
		strs["a"], ints[0] = "changed", 2
		nested[1][0].(map[string]int)["a"] = 2
		testutil.Equal(t, "s", c.Get("s"), any(map[string]string{"a": "x"}))
		testutil.Equal(t, "i", c.Get("i"), any([]int{1}))
		testutil.Equal(t, "n", c.Get("n"), any(map[int][]any{1: {map[string]int{"a": 1}, nil}}))
		testutil.Equal(t, "nil", c.Get("nil"), any(map[string]int(nil)))
		testutil.Equal(t, "nils", c.Get("nils"), any([]int(nil)))

		got := c.Get("n").(map[int][]any)
		got[1][0].(map[string]int)["a"] = 3
		testutil.Equal(t, "n after a change of a copy", c.Get("n"), any(map[int][]any{1: {map[string]int{"a": 1}, nil}}))
	})

	t.Run("should override the defaults and every source", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1}, source("s", 100, container.Bag{"a": 2}))
		testutil.NoError(t, c.Set("a", 3))
		testutil.Equal(t, "a", c.Int("a"), 3)
		testutil.NoError(t, c.AddSource(source("t", 200, container.Bag{"a": 4})))
		testutil.Equal(t, "a after a source", c.Int("a"), 3)
	})

	t.Run("should merge a stored bag with the lower layers", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": container.Bag{"b": 1, "c": 1}})
		testutil.NoError(t, c.Set("a", container.Bag{"c": 2}))
		testutil.Equal(t, "a", c.Get("a"), any(container.Bag{"b": 1, "c": 2}))
	})

	t.Run("should replace a stored value", func(t *testing.T) {
		c := emptyConfig()
		testutil.NoError(t, c.Set("a.b", 1))
		testutil.NoError(t, c.Set("a", 2))
		testutil.Equal(t, "a", c.Get("a"), any(2))
	})

	t.Run("should return the error of the bag, unchanged", func(t *testing.T) {
		c := emptyConfig()
		testutil.NoError(t, c.Set("a", 1))
		testutil.Error(t, c.Set("a..b", 1), container.ErrInvalidPath,
			"unable to set bag value: invalid path", map[string]any{"path": "a..b"})
		testutil.Error(t, c.Set("", 1), container.ErrInvalidPath,
			"unable to set bag value: invalid path", map[string]any{"path": ""})
		testutil.Error(t, c.Set("a.b", 1), container.ErrPathConflict,
			"unable to set bag value: path conflict", map[string]any{"path": "a.b", "segment": "a"})
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 1}))
	})

	t.Run("should refuse a value holding an invalid key, unchanged", func(t *testing.T) {
		c := emptyConfig()
		err := c.Set("a", container.Bag{"b..c": 1})
		testutil.Error(t, err, container.ErrInvalidPath,
			"unable to merge bag: invalid path", map[string]any{"path": "", "key": "a.b..c"})
		testutil.Equal(t, "Has()", c.Has("a"), false)
	})

	t.Run("should not take a path conflict with a lower layer as an error", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1})
		testutil.NoError(t, c.Set("a.b", 2))
		testutil.Equal(t, "a", c.Get("a"), any(container.Bag{"b": 2}))
	})
}

// ---------------------------------------------------------------------------
// Populate
// ---------------------------------------------------------------------------

func Test_Populate(t *testing.T) {
	type server struct {
		Host string `json:"host"`
		Port int
	}
	type target struct {
		Server server `json:"server"`
		Debug  bool
	}

	c := newConfig(t, container.Bag{"server": container.Bag{"host": "h", "port": 80}, "debug": true})

	t.Run("should decode the whole configuration", func(t *testing.T) {
		var got target
		testutil.NoError(t, c.Populate(&got))
		testutil.Equal(t, "target", got, target{Server: server{Host: "h", Port: 80}, Debug: true})
	})

	t.Run("should decode a path", func(t *testing.T) {
		var got server
		testutil.NoError(t, c.PopulatePath("server", &got))
		testutil.Equal(t, "target", got, server{Host: "h", Port: 80})

		var port int
		testutil.NoError(t, c.PopulatePath("server.port", &port))
		testutil.Equal(t, "port", port, 80)
	})

	for name, invalid := range map[string]any{
		"nil": nil, "a value": target{}, "a nil pointer": (*target)(nil), "a map": map[string]any{},
	} {
		t.Run("should refuse "+name+" target", func(t *testing.T) {
			testutil.Error(t, c.Populate(invalid), config.ErrInvalidTarget,
				"unable to populate from config: invalid config populate target", map[string]any{})
			testutil.Error(t, c.PopulatePath("server", invalid), config.ErrInvalidTarget,
				"unable to populate from config path: invalid config populate target",
				map[string]any{"path": "server"})
		})
	}

	t.Run("should fail for a missing path", func(t *testing.T) {
		var got server
		testutil.Error(t, c.PopulatePath("client", &got), config.ErrMissingPath,
			"unable to populate from config path: config path not found", map[string]any{"path": "client"})
	})

	t.Run("should return the json error", func(t *testing.T) {
		var port string
		err := c.PopulatePath("server.port", &port)
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("PopulatePath() = %v, want a json.UnmarshalTypeError", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("PopulatePath() = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{"path": "server.port"})

		channel := newConfig(t, container.Bag{"a": make(chan int)})
		var got map[string]any
		var typeOfErr *json.UnsupportedTypeError
		if err := channel.Populate(&got); !errors.As(err, &typeOfErr) {
			t.Fatalf("Populate() = %v, want a json.UnsupportedTypeError", err)
		}
	})

	t.Run("should decode the floats JSON cannot hold as null", func(t *testing.T) {
		c := newConfig(t, container.Bag{
			"db":     container.Bag{"host": "h"},
			"limits": container.Bag{"max": math.Inf(1), "min": float32(math.Inf(-1)), "list": []any{math.NaN(), 1.5}},
			"raw":    []byte("x"),
			"ptr":    (*int)(nil),
		})
		var got struct {
			Db     struct{ Host string }
			Limits struct {
				Max  float64
				Min  float64
				List []any
			}
			Raw []byte
		}
		got.Limits.Max = 3
		testutil.NoError(t, c.Populate(&got))
		testutil.Equal(t, "Host", got.Db.Host, "h")
		testutil.Equal(t, "Max unchanged", got.Limits.Max, 3.0)
		testutil.Equal(t, "Min", got.Limits.Min, 0.0)
		testutil.Equal(t, "List", got.Limits.List, []any{nil, 1.5})
		testutil.Equal(t, "Raw", got.Raw, []byte("x"))

		testutil.NoError(t, c.Set("none", nil))
		target := 5
		testutil.NoError(t, c.PopulatePath("none", &target))
		testutil.Equal(t, "nil leaves the target", target, 5)
	})
}

// ---------------------------------------------------------------------------
// sources
// ---------------------------------------------------------------------------

func Test_Sources(t *testing.T) {
	t.Run("should merge the sources by priority, then id", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 0, "b": 0, "c": 0, "d": 0},
			source("high", 10, container.Bag{"a": "high"}),
			source("low", -10, container.Bag{"a": "low", "b": "low"}),
			source("y", 0, container.Bag{"a": "y", "b": "y", "c": "y"}),
			source("x", 0, container.Bag{"a": "x", "b": "x", "c": "x"}),
		)
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": "high", "b": "y", "c": "y", "d": 0}))
		testutil.Equal(t, "HasSource()", c.HasSource("x"), true)
	})

	t.Run("should merge the nested bags and expand the dotted keys", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": container.Bag{"b": 1, "c": 1}},
			source("s", 0, container.Bag{"a.c": 2, "a.d": 2}))
		testutil.Equal(t, "a", c.Get("a"), any(container.Bag{"b": 1, "c": 2, "d": 2}))
	})

	t.Run("should keep the values of a source as they were when added", func(t *testing.T) {
		s := source("s", 0, container.Bag{"a": 1})
		c := newConfig(t, nil, s)
		s.set(container.Bag{"a": 2})
		testutil.Equal(t, "a", c.Int("a"), 1)
	})

	t.Run("should accept a source with no values", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1}, source("s", 0, nil))
		testutil.Equal(t, "a", c.Int("a"), 1)
	})

	t.Run("should remove a source, without closing it", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 0}, source("s", 0, container.Bag{"a": 1, "b": 1}))
		testutil.NoError(t, c.RemoveSource("s"))
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 0}))
		testutil.Equal(t, "HasSource()", c.HasSource("s"), false)
		testutil.NoError(t, c.AddSource(source("s", 0, container.Bag{"a": 2})))
		testutil.Equal(t, "a", c.Int("a"), 2)
	})

	for name, nilSource := range map[string]flam.IConfigSource{"nil": nil, "nil pointer": (*fakeSource)(nil)} {
		t.Run("should refuse a "+name+" source", func(t *testing.T) {
			testutil.Error(t, emptyConfig().AddSource(nilSource), config.ErrNilSource,
				"unable to add config source: nil config source", map[string]any{})
		})
	}

	t.Run("should refuse a duplicate source, unchanged", func(t *testing.T) {
		c := newConfig(t, nil, source("s", 0, container.Bag{"a": 1}))
		testutil.Error(t, c.AddSource(source("s", 1, container.Bag{"a": 2})), config.ErrDuplicateSource,
			"unable to add config source: config source id already in use", map[string]any{"id": "s"})
		testutil.Equal(t, "a", c.Int("a"), 1)
	})

	t.Run("should refuse a source holding an invalid key", func(t *testing.T) {
		c := emptyConfig()
		err := c.AddSource(source("s", 0, container.Bag{"a..b": 1}))
		testutil.Error(t, err, container.ErrInvalidPath,
			"unable to add config source: unable to merge bag: invalid path",
			map[string]any{"id": "s"})
		testutil.Equal(t, "HasSource()", c.HasSource("s"), false)

		err = c.AddSource(&fakeSource{id: "f", bag: failingBag{}})
		if !errors.Is(err, container.ErrUnsupportedBag) {
			t.Fatalf("AddSource() = %v, want container.ErrUnsupportedBag", err)
		}
	})

	t.Run("should refuse to remove an unknown source", func(t *testing.T) {
		testutil.Error(t, emptyConfig().RemoveSource("s"), config.ErrUnknownSource,
			"unable to remove config source: unknown config source id", map[string]any{"id": "s"})
	})
}

func Test_ReloadSources(t *testing.T) {
	t.Run("should merge the values of the sources changed", func(t *testing.T) {
		s := reloadable("s", 0, container.Bag{"a": 1, "b": 1}, container.Bag{"a": 2})
		c := newConfig(t, container.Bag{"b": 0}, s, source("static", -1, container.Bag{"c": 1}))
		var r recorder
		r.observe(t, c, "a", "a")
		r.observe(t, c, "b", "b")
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 2, "b": 0, "c": 1}))
		testutil.Equal(t, "calls", r.take(), []change{{"a", 1, 2}, {"b", 1, 0}})

		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "calls when unchanged", r.take(), []change(nil))
	})

	t.Run("should keep the values of the failed sources, and return their errors joined", func(t *testing.T) {
		first, second := errors.New("first"), errors.New("second")
		failing := func(id string, err error) *reloadSource {
			return &reloadSource{fakeSource: source(id, 0, container.Bag{id: 1}),
				reload: func(s *reloadSource) (bool, error) {
					s.set(container.Bag{id: 2})
					return false, err
				}}
		}
		ok := reloadable("ok", 0, container.Bag{"ok": 1}, container.Bag{"ok": 2})
		c := newConfig(t, nil, failing("b", second), ok, failing("a", first))
		err := c.ReloadSources()
		if !errors.Is(err, first) || !errors.Is(err, second) {
			t.Fatalf("ReloadSources() = %v, want first and second joined", err)
		}
		testutil.Equal(t, "Error()", err.Error(),
			"unable to reload config source: first\nunable to reload config source: second")
		var e *flam.Error
		for i, id := range []string{"a", "b"} {
			if !errors.As(joinedErrors(err)[i], &e) {
				t.Fatalf("error %d = %T, want a *flam.Error", i, joinedErrors(err)[i])
			}
			testutil.Context(t, e, map[string]any{config.ErrorFieldID: id})
		}
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 1, "b": 1, "ok": 2}))
	})

	t.Run("should keep the values of a source reloaded with an invalid key", func(t *testing.T) {
		s := reloadable("s", 0, container.Bag{"a": 1}, container.Bag{"a..b": 2})
		c := newConfig(t, nil, s)
		testutil.Error(t, c.ReloadSources(), container.ErrInvalidPath,
			"unable to reload config source: unable to merge bag: invalid path",
			map[string]any{"id": "s"})
		testutil.Equal(t, "Get()", c.Get(""), any(container.Bag{"a": 1}))
	})

	t.Run("should skip a source removed while reloading", func(t *testing.T) {
		c := emptyConfig()
		s := &reloadSource{fakeSource: source("s", 0, container.Bag{"a": 1})}
		s.reload = func(s *reloadSource) (bool, error) {
			testutil.NoError(t, c.RemoveSource("s"))
			testutil.NoError(t, c.AddSource(source("s", 0, container.Bag{"a": 3})))
			s.set(container.Bag{"a": 2})
			return true, nil
		}
		testutil.NoError(t, c.AddSource(s))
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "a", c.Int("a"), 3)
	})

	t.Run("should reload nothing without reloadable source", func(t *testing.T) {
		c := newConfig(t, nil, source("s", 0, container.Bag{"a": 1}))
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "a", c.Int("a"), 1)
	})
}

// ---------------------------------------------------------------------------
// observers
// ---------------------------------------------------------------------------

func Test_Observers(t *testing.T) {
	t.Run("should add and remove observers", func(t *testing.T) {
		c := emptyConfig()
		noop := func(any, any) {}
		testutil.NoError(t, c.AddObserver("o", "a.b", noop))
		testutil.Equal(t, "HasObserver()", c.HasObserver("o", "a.b"), true)
		testutil.Equal(t, "HasObserver(other path)", c.HasObserver("o", "a"), false)
		testutil.Error(t, c.AddObserver("o", "c", noop), config.ErrDuplicateObserver,
			"unable to add config observer: config observer id already in use", map[string]any{"id": "o"})
		testutil.NoError(t, c.RemoveObserver("o"))
		testutil.Equal(t, "HasObserver() after removal", c.HasObserver("o", "a.b"), false)
		testutil.Error(t, c.RemoveObserver("o"), config.ErrUnknownObserver,
			"unable to remove config observer: unknown config observer id", map[string]any{"id": "o"})
	})

	t.Run("should refuse a nil callback or a malformed path", func(t *testing.T) {
		c := emptyConfig()
		testutil.Error(t, c.AddObserver("o", "a", nil), config.ErrNilObserver,
			"unable to add config observer: nil config observer", map[string]any{"id": "o", "path": "a"})
		for _, path := range []string{"a..b", ".a", "a."} {
			testutil.Error(t, c.AddObserver("o", path, func(any, any) {}), container.ErrInvalidPath,
				"unable to add config observer: invalid path", map[string]any{"id": "o", "path": path})
		}
		testutil.Equal(t, "HasObserver()", c.HasObserver("o", "a"), false)
	})

	t.Run("should call the observers of a change at, under or above their path", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": container.Bag{"b": 1, "c": 1}})
		var r recorder
		r.observe(t, c, "at", "a.b")
		r.observe(t, c, "above", "a")
		r.observe(t, c, "under", "a.b.x")
		r.observe(t, c, "other", "a.c")
		r.observe(t, c, "whole", "")

		testutil.NoError(t, c.Set("a.b", 2))
		testutil.Equal(t, "calls", r.take(), []change{
			{"above", container.Bag{"b": 1, "c": 1}, container.Bag{"b": 2, "c": 1}},
			{"at", 1, 2},
			{"whole", container.Bag{"a": container.Bag{"b": 1, "c": 1}}, container.Bag{"a": container.Bag{"b": 2, "c": 1}}},
		})

		testutil.NoError(t, c.Set("a.b", container.Bag{"x": 1}))
		calls := r.take()
		testutil.Equal(t, "ids", []string{calls[0].id, calls[1].id, calls[2].id, calls[3].id},
			[]string{"above", "at", "under", "whole"})
		testutil.Equal(t, "under", calls[2], change{"under", nil, 1})
	})

	t.Run("should call no observer for an equal value", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1, "n": math.NaN(), "e": container.Bag{}})
		var r recorder
		r.observe(t, c, "a", "a")
		r.observe(t, c, "n", "n")
		r.observe(t, c, "e", "e")
		testutil.NoError(t, c.Set("a", 1))
		testutil.NoError(t, c.Set("b", 1))
		testutil.NoError(t, c.AddSource(source("s", 0, container.Bag{"e": container.Bag(nil)})))
		testutil.Equal(t, "calls", r.take(), []change(nil))
	})

	t.Run("should call the observers of a source change", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": 1})
		var r recorder
		r.observe(t, c, "a", "a")
		testutil.NoError(t, c.AddSource(source("s", 0, container.Bag{"a": 2})))
		testutil.NoError(t, c.RemoveSource("s"))
		testutil.Equal(t, "calls", r.take(), []change{{"a", 1, 2}, {"a", 2, 1}})
	})

	t.Run("should give copies to each observer", func(t *testing.T) {
		c := newConfig(t, container.Bag{"a": container.Bag{"b": 1}})
		for _, id := range []string{"x", "y"} {
			testutil.NoError(t, c.AddObserver(id, "a", func(old, newValue any) {
				testutil.Equal(t, id+" old", old, any(container.Bag{"b": 1}))
				testutil.Equal(t, id+" new", newValue, any(container.Bag{"b": 2}))
				old.(container.Bag)["b"] = 5
				newValue.(container.Bag)["b"] = 5
			}))
		}
		testutil.NoError(t, c.Set("a.b", 2))
		testutil.Equal(t, "a.b", c.Int("a.b"), 2)
	})

	t.Run("should call the observers once released, before a nested Set returns", func(t *testing.T) {
		c := emptyConfig()
		var order []string
		testutil.NoError(t, c.AddObserver("a", "a", func(_, newValue any) {
			order = append(order, fmt.Sprint("a=", newValue, " b=", c.Int("b")))
			if newValue == 1 {
				testutil.NoError(t, c.Set("b", 2))
				order = append(order, "nested Set returned")
			}
		}))
		testutil.NoError(t, c.AddObserver("b", "b", func(_, newValue any) {
			order = append(order, fmt.Sprint("b=", newValue))
		}))
		testutil.NoError(t, c.Set("a", 1))
		testutil.Equal(t, "order", order, []string{"a=1 b=0", "b=2", "nested Set returned"})
	})

	t.Run("should not call an observer for a failed change", func(t *testing.T) {
		c := emptyConfig()
		var r recorder
		r.observe(t, c, "o", "")
		_ = c.Set("a..b", 1)
		_ = c.RemoveSource("s")
		testutil.Equal(t, "calls", r.take(), []change(nil))
	})
}

// ---------------------------------------------------------------------------
// concurrency
// ---------------------------------------------------------------------------

func Test_Concurrency(t *testing.T) {
	s := reloadable("s", 0, container.Bag{"r": 0}, container.Bag{"r": 1}, container.Bag{"r": 2})
	c := newConfig(t, container.Bag{"d": 0}, s)
	var calls sync.Map
	testutil.NoError(t, c.AddObserver("o", "", func(_, newValue any) {
		calls.Store(fmt.Sprint(newValue), true)
	}))

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() { testutil.NoError(t, c.Set(fmt.Sprintf("k%d", i%5), i)) })
		wg.Go(func() { _ = c.ReloadSources() })
		wg.Go(func() { _ = c.AddDefaults(container.Bag{fmt.Sprintf("d%d", i): i}) })
		wg.Go(func() {
			id := fmt.Sprintf("s%d", i)
			testutil.NoError(t, c.AddSource(source(id, i, container.Bag{"x": i})))
			testutil.NoError(t, c.RemoveSource(id))
		})
		wg.Go(func() {
			_ = c.Get("")
			_ = c.Entries()
			_ = c.Bag("")
			_ = c.Int("k1")
		})
	}
	wg.Wait()

	entries := c.Entries()
	for i := range 5 {
		if !slices.Contains(entries, fmt.Sprintf("k%d", i)) {
			t.Errorf("Entries() = %v, missing k%d", entries, i)
		}
	}
	testutil.Equal(t, "r", c.Int("r"), 2)
	testutil.Equal(t, "x", c.Has("x"), false)
	if !strings.HasPrefix(fmt.Sprint(c.Get("d")), "0") {
		t.Errorf("d = %v, want 0", c.Get("d"))
	}
}

// ---------------------------------------------------------------------------
// regressions
// ---------------------------------------------------------------------------

// joinedErrors returns the errors joined in err.
func joinedErrors(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	return []error{err}
}

// priorityReader is a source whose Priority reads the configuration.
type priorityReader struct {
	*fakeSource
	config flam.IConfig
}

func (s *priorityReader) Priority() int { return s.config.Int("priority") }

// within fails the test unless fn returns within a few seconds.
func within(t *testing.T, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not return: deadlock", name)
	}
}

func Test_Regressions(t *testing.T) {
	t.Run("should let an observer reload the sources", func(t *testing.T) {
		s := reloadable("s", 0, container.Bag{"a": 1}, container.Bag{"a": 2}, container.Bag{"a": 3})
		c := newConfig(t, nil, s)
		reloads := 0
		testutil.NoError(t, c.AddObserver("o", "a", func(_, _ any) {
			if reloads++; reloads == 1 {
				testutil.NoError(t, c.ReloadSources())
			}
		}))
		within(t, "ReloadSources()", func() { testutil.NoError(t, c.ReloadSources()) })
		testutil.Equal(t, "a", c.Int("a"), 3)
		testutil.Equal(t, "reloads", reloads, 2)
	})

	t.Run("should set a path under a key holding dots set before", func(t *testing.T) {
		c := newConfig(t, nil)
		testutil.NoError(t, c.Set("a", map[string]any{"b.c": 1}))
		testutil.Equal(t, "a.b.c", c.Int("a.b.c"), 1)
		testutil.NoError(t, c.Set("a.b.c", 2))
		testutil.Equal(t, "a.b.c", c.Int("a.b.c"), 2)
		testutil.NoError(t, c.Set("a.b", 5))
		testutil.Equal(t, "a.b", c.Int("a.b"), 5)
	})

	t.Run("should not call the observers for an equal bag of another map type", func(t *testing.T) {
		c := newConfig(t, nil)
		testutil.NoError(t, c.Set("a", map[string]any{"x": 1}))
		r := &recorder{}
		r.observe(t, c, "o", "a")
		testutil.NoError(t, c.Set("a", container.Bag{"x": 1}))
		testutil.NoError(t, c.AddSource(source("s", 0, container.Bag{"a": container.Bag{"x": 1}})))
		testutil.Equal(t, "calls", r.take(), []change(nil))
	})

	t.Run("should order the extreme priorities", func(t *testing.T) {
		c := newConfig(t, nil,
			source("high", math.MaxInt, container.Bag{"a": "high"}),
			source("low", math.MinInt, container.Bag{"a": "low"}),
		)
		testutil.Equal(t, "a", c.String("a"), "high")
	})

	t.Run("should read the priority of a source before holding the configuration", func(t *testing.T) {
		c := newConfig(t, container.Bag{"priority": 1})
		s := &priorityReader{fakeSource: source("s", 0, container.Bag{"a": 1}), config: c}
		within(t, "AddSource()", func() { testutil.NoError(t, c.AddSource(s)) })
		testutil.Equal(t, "a", c.Int("a"), 1)
	})

	t.Run("should keep the values reloaded by a source added again meanwhile", func(t *testing.T) {
		var c flam.IConfig
		s := &reloadSource{fakeSource: source("s", 0, container.Bag{"a": 1})}
		s.reload = func(s *reloadSource) (bool, error) {
			testutil.NoError(t, c.RemoveSource("s"))
			testutil.NoError(t, c.AddSource(s)) // with the values before the reload
			s.set(container.Bag{"a": 2})
			return true, nil
		}
		c = newConfig(t, nil, s)
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "a", c.Int("a"), 2)
	})

	t.Run("should skip the values reloaded by a source replaced meanwhile", func(t *testing.T) {
		var c flam.IConfig
		other := source("s", 0, container.Bag{"a": 3})
		s := &reloadSource{fakeSource: source("s", 0, container.Bag{"a": 1})}
		s.reload = func(s *reloadSource) (bool, error) {
			testutil.NoError(t, c.RemoveSource("s"))
			testutil.NoError(t, c.AddSource(other))
			s.set(container.Bag{"a": 2})
			return true, nil
		}
		c = newConfig(t, nil, s)
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "a", c.Int("a"), 3)
	})

	t.Run("should reload a source of a type that cannot be compared", func(t *testing.T) {
		s := uncomparable{reloadSource: reloadable("s", 0, container.Bag{"a": 1}, container.Bag{"a": 2})}
		c := newConfig(t, nil, s)
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "a", c.Int("a"), 2)
	})

	t.Run("should skip the values reloaded by a source of a type that cannot be compared, added again", func(t *testing.T) {
		var c flam.IConfig
		s := uncomparable{reloadSource: &reloadSource{fakeSource: source("s", 0, container.Bag{"a": 1})}}
		s.reload = func(r *reloadSource) (bool, error) {
			testutil.NoError(t, c.RemoveSource("s"))
			testutil.NoError(t, c.AddSource(s))
			r.set(container.Bag{"a": 2})
			return true, nil
		}
		c = newConfig(t, nil, s)
		testutil.NoError(t, c.ReloadSources())
		testutil.Equal(t, "a", c.Int("a"), 1)
	})
}

// holding is a reloadable source of a comparable type holding a value that
// cannot be compared.
type holding struct {
	*reloadSource
	extra any
}

func Test_ReloadSources_Holding(t *testing.T) {
	t.Run("should skip the values reloaded by a source holding a value that cannot be compared, added again", func(t *testing.T) {
		var c flam.IConfig
		s := holding{reloadSource: &reloadSource{fakeSource: source("s", 0, container.Bag{"a": 1})}, extra: map[string]int{}}
		s.reload = func(r *reloadSource) (bool, error) {
			testutil.NoError(t, c.RemoveSource("s"))
			testutil.NoError(t, c.AddSource(s))
			r.set(container.Bag{"a": 2})
			return true, nil
		}
		c = newConfig(t, nil, s)
		testutil.NotPanics(t, "ReloadSources()", func() { testutil.NoError(t, c.ReloadSources()) })
		testutil.Equal(t, "a", c.Int("a"), 1)
	})
}

// uncomparable is a reloadable source of a type that cannot be compared.
type uncomparable struct {
	*reloadSource
	_ []int
}
