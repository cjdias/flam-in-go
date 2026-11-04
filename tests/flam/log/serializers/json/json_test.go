package json

import (
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// creators returns the serializer creators the bundle provides in the
// serializer creator group.
func creators(t *testing.T) []flam.ILogSerializerCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, json.NewBundle().Register(app))

	var result []flam.ILogSerializerCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.ILogSerializerCreator `group:"flam.log.serializers.creator"`
	}) {
		result = args.Creators
	}))
	return result
}

// newSerializer returns the serializer of the id created by a factory
// configured with the defaults of the bundle and with the creators it
// provides, the id being configured with the fields when not the default.
func newSerializer(t *testing.T, id string, fields container.Bag) flam.ILogSerializer {
	t.Helper()
	bag := container.Bag{}
	testutil.NoError(t, json.NewBundle().(flam.IConfigurableBundle).Config(bag))
	if id != json.Id {
		fields[factory.ConfigFieldDriver] = json.Driver
		testutil.NoError(t, bag.Set(log.SerializerConfigPath+"."+id, fields))
	}

	f, err := factory.New(bag.Bag(log.SerializerConfigPath), creators(t)...)
	testutil.NoError(t, err)
	s, err := f.Get(id)
	testutil.NoError(t, err)
	return s
}

// ts is the timestamp of the serialized entries.
var ts = time.Date(2026, 11, 2, 9, 43, 0, 123_000_000, time.FixedZone("", 3600))

// serialize serializes an info entry with the default serializer.
func serialize(t *testing.T, message string, ctx flam.IBag) string {
	t.Helper()
	return newSerializer(t, json.Id, nil).Serialize(ts, flam.LogInfo, message, ctx)
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", json.NewBundle().Id(), "flam.log.serializers.json")
	})

	t.Run("should configure the default serializer", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, json.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag, container.Bag{"flam": container.Bag{"log": container.Bag{
			"serializers": container.Bag{"json": container.Bag{"driver": json.Driver}},
		}}})
	})

	t.Run("should provide its creator in the serializer creator group", func(t *testing.T) {
		testutil.Equal(t, "creators", len(creators(t)), 1)
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		boom := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return boom }
		if err := json.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept only its driver", func(t *testing.T) {
		c := creators(t)[0]
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": json.Driver}), true)
		testutil.Equal(t, "Accept(other)", c.Accept(container.Bag{"driver": "other"}), false)
		testutil.Equal(t, "Accept(none)", c.Accept(container.Bag{}), false)
	})

	t.Run("should create a serializer of the id, with nothing to close", func(t *testing.T) {
		s := newSerializer(t, "other", container.Bag{})
		testutil.Equal(t, "Id()", s.Id(), "other")
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
	})

	t.Run("should format the timestamp with the configured layout", func(t *testing.T) {
		s := newSerializer(t, "other", container.Bag{json.ConfigFieldFormat: time.Kitchen})
		testutil.Equal(t, "line", s.Serialize(ts, flam.LogInfo, "m", nil), `{"level":"INFO","message":"m","time":"9:43AM"}`)
	})

	t.Run("should use the default layout for an empty one", func(t *testing.T) {
		s := newSerializer(t, "other", container.Bag{json.ConfigFieldFormat: ""})
		testutil.Equal(t, "line", s.Serialize(ts, flam.LogInfo, "m", nil), `{"level":"INFO","message":"m","time":"2026-11-02T09:43:00.123+0100"}`)
	})
}

// ---------------------------------------------------------------------------
// serializer
// ---------------------------------------------------------------------------

// prefix is the start of the line of an info entry "m" at ts, without
// context.
const prefix = `{"level":"INFO","message":"m","time":"2026-11-02T09:43:00.123+0100"`

func Test_Serializer(t *testing.T) {
	t.Run("should write the timestamp, the level and the message", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", nil), prefix+"}")
	})

	t.Run("should write each level in upper case", func(t *testing.T) {
		s := newSerializer(t, json.Id, nil)
		for level, name := range map[flam.LogLevel]string{
			flam.LogNone: "NONE", flam.LogFatal: "FATAL", flam.LogError: "ERROR", flam.LogWarning: "WARNING",
			flam.LogNotice: "NOTICE", flam.LogInfo: "INFO", flam.LogDebug: "DEBUG",
		} {
			testutil.Equal(t, name, s.Serialize(ts, level, "m", nil),
				`{"level":"`+name+`","message":"m","time":"2026-11-02T09:43:00.123+0100"}`)
		}
	})

	t.Run("should write the context values, in key order", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{"b": 2, "a": "x", "z": true, "n": nil, "f": 1.5}),
			`{"a":"x","b":2,"f":1.5,"level":"INFO","message":"m","n":null,"time":"2026-11-02T09:43:00.123+0100","z":true}`)
	})

	t.Run("should write nothing more for an empty context", func(t *testing.T) {
		testutil.Equal(t, "empty", serialize(t, "m", container.Bag{}), prefix+"}")
		var nilBag container.Bag
		testutil.Equal(t, "nil", serialize(t, "m", nilBag), prefix+"}")
	})

	t.Run("should set the entry fields over the context values", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{"level": "x", "message": "x", "time": "x"}), prefix+"}")
	})

	t.Run("should nest the bags, and convert the errors and the items", func(t *testing.T) {
		var nilBag container.Bag
		var nilErr *flam.Error
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{
			"db":   container.Bag{"host": "x", "pool": container.Bag{"size": 3}},
			"err":  errors.New("failed"),
			"list": []any{1, container.Bag{"k": "v"}, errors.New("e")},
			"nb":   nilBag,
			"ne":   nilErr,
		}), `{"db":{"host":"x","pool":{"size":3}},"err":"failed",`+
			`"level":"INFO","list":[1,{"k":"v"},"e"],"message":"m","nb":null,"ne":null,`+
			`"time":"2026-11-02T09:43:00.123+0100"}`)
	})

	t.Run("should convert the values at any depth", func(t *testing.T) {
		type named map[string]int
		var nilErrs []error
		var nilMap map[string]any
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{
			"errs":  []error{errors.New("e1"), nil},
			"map":   map[string]any{"e": errors.New("e2"), "n": named{"x": 1}},
			"array": [2]any{errors.New("e3"), 1},
			"bytes": []byte("hi"),
			"time":  at,
			"keys":  map[int]string{1: "a"},
			"nils":  []any{nilErrs, nilMap},
		}), `{"array":["e3",1],"bytes":"aGk=","errs":["e1",null],"keys":{"1":"a"},"level":"INFO",`+
			`"map":{"e":"e2","n":{"x":1}},"message":"m","nils":[null,null],`+
			`"time":"2026-11-02T09:43:00.123+0100"}`)
	})

	t.Run("should keep the values encoding themselves", func(t *testing.T) {
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{"at": at, "list": []time.Time{at}}),
			`{"at":"2026-01-02T03:04:05Z","level":"INFO","list":["2026-01-02T03:04:05Z"],"message":"m",`+
				`"time":"2026-11-02T09:43:00.123+0100"}`)
	})

	t.Run("should escape the message, but not the HTML characters", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "<a & \"b\">\n", nil),
			`{"level":"INFO","message":"<a & \"b\">\n","time":"2026-11-02T09:43:00.123+0100"}`)
	})

	for name, val := range map[string]any{"channel": make(chan int), "NaN": math.NaN()} {
		t.Run("should leave out a context holding a "+name, func(t *testing.T) {
			got := serialize(t, "m", container.Bag{"a": 1, "bad": val})
			if !strings.HasPrefix(got, `{"context_error":"json: unsupported `) ||
				!strings.HasSuffix(got, `","level":"INFO","message":"m","time":"2026-11-02T09:43:00.123+0100"}`) ||
				strings.Contains(got, `"a":`) {
				t.Fatalf("line = %s", got)
			}
		})
	}

	t.Run("should not change the context", func(t *testing.T) {
		ctx := container.Bag{"a": container.Bag{"b": 1}, "time": "x"}
		_ = serialize(t, "m", ctx)
		testutil.Equal(t, "ctx", ctx, container.Bag{"a": container.Bag{"b": 1}, "time": "x"})
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		s := newSerializer(t, json.Id, nil)
		ctx := container.Bag{"a": 1}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 100 {
					if got := s.Serialize(ts, flam.LogInfo, "m", ctx); got != `{"a":1,`+prefix[1:]+"}" {
						t.Errorf("line = %q", got)
					}
				}
			})
		}
		wg.Wait()
	})
}
