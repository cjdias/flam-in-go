package stringify

import (
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify"
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
	testutil.NoError(t, stringify.NewBundle().Register(app))

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
	testutil.NoError(t, stringify.NewBundle().(flam.IConfigurableBundle).Config(bag))
	if id != stringify.Id {
		fields[factory.ConfigFieldDriver] = stringify.Driver
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
	return newSerializer(t, stringify.Id, nil).Serialize(ts, flam.LogInfo, message, ctx)
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", stringify.NewBundle().Id(), "flam.log.serializers.stringify")
	})

	t.Run("should configure the default serializer", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, stringify.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag, container.Bag{"flam": container.Bag{"log": container.Bag{
			"serializers": container.Bag{"stringify": container.Bag{"driver": stringify.Driver}},
		}}})
	})

	t.Run("should provide its creator in the serializer creator group", func(t *testing.T) {
		testutil.Equal(t, "creators", len(creators(t)), 1)
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		boom := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return boom }
		if err := stringify.NewBundle().Register(app); !errors.Is(err, boom) {
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
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": stringify.Driver}), true)
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
		s := newSerializer(t, "other", container.Bag{stringify.ConfigFieldFormat: time.Kitchen})
		testutil.Equal(t, "line", s.Serialize(ts, flam.LogInfo, "m", nil), "9:43AM [INFO] m")
	})

	t.Run("should use the default layout for an empty one", func(t *testing.T) {
		s := newSerializer(t, "other", container.Bag{stringify.ConfigFieldFormat: ""})
		testutil.Equal(t, "line", s.Serialize(ts, flam.LogInfo, "m", nil), "2026-11-02T09:43:00.123+0100 [INFO] m")
	})
}

// ---------------------------------------------------------------------------
// serializer
// ---------------------------------------------------------------------------

func Test_Serializer(t *testing.T) {
	t.Run("should write the timestamp, the level and the message", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "unable to connect", nil),
			"2026-11-02T09:43:00.123+0100 [INFO] unable to connect")
	})

	t.Run("should write each level in upper case", func(t *testing.T) {
		s := newSerializer(t, stringify.Id, nil)
		for level, name := range map[flam.LogLevel]string{
			flam.LogNone: "NONE", flam.LogFatal: "FATAL", flam.LogError: "ERROR", flam.LogWarning: "WARNING",
			flam.LogNotice: "NOTICE", flam.LogInfo: "INFO", flam.LogDebug: "DEBUG",
		} {
			testutil.Equal(t, name, s.Serialize(ts, level, "m", nil), "2026-11-02T09:43:00.123+0100 ["+name+"] m")
		}
	})

	t.Run("should write the context pairs in key order", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{"b": 2, "a": "x", "c": true}),
			"2026-11-02T09:43:00.123+0100 [INFO] m a=x b=2 c=true")
	})

	t.Run("should write nothing for an empty context", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{}), "2026-11-02T09:43:00.123+0100 [INFO] m")
		var nilBag container.Bag
		testutil.Equal(t, "line", serialize(t, "m", nilBag), "2026-11-02T09:43:00.123+0100 [INFO] m")
	})

	t.Run("should flatten the nested bags and maps, but the empty and nil ones", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{
			"db": container.Bag{"host": "x", "pool": map[string]any{"size": 3}}, "e": container.Bag{},
			"n": container.Bag(nil), "nm": map[string]any(nil), "z": 1,
		}), "2026-11-02T09:43:00.123+0100 [INFO] m db.host=x db.pool.size=3 z=1")
	})

	t.Run("should format the values as fmt.Sprint", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{
			"err": errors.New("failed"), "f": 1.5, "nil": nil, "list": []any{1, 2},
		}), "2026-11-02T09:43:00.123+0100 [INFO] m err=failed f=1.5 list=\"[1 2]\" nil=<nil>")
	})

	t.Run("should quote the keys and values needing it", func(t *testing.T) {
		testutil.Equal(t, "line", serialize(t, "m", container.Bag{
			"a": "", "b": "x y", "c": `say "hi"`, "d": "k=v", "e": "line\nbreak", "f key": 1, "g": "ünïcode",
		}), `2026-11-02T09:43:00.123+0100 [INFO] m a="" b="x y" c="say \"hi\"" d="k=v" e="line\nbreak" "f key"=1 g=ünïcode`)
	})

	t.Run("should quote a message holding a quote, an equal sign or a character that is not printable", func(t *testing.T) {
		testutil.Equal(t, "plain", serialize(t, "a: b, c", nil), `2026-11-02T09:43:00.123+0100 [INFO] a: b, c`)
		testutil.Equal(t, "quote", serialize(t, `a "b"`, nil), `2026-11-02T09:43:00.123+0100 [INFO] "a \"b\""`)
		testutil.Equal(t, "equal", serialize(t, "x user=admin", nil), `2026-11-02T09:43:00.123+0100 [INFO] "x user=admin"`)
		testutil.Equal(t, "quoted", serialize(t, "a\nb\tc", nil), `2026-11-02T09:43:00.123+0100 [INFO] "a\nb\tc"`)
		testutil.Equal(t, "empty", serialize(t, "", nil), `2026-11-02T09:43:00.123+0100 [INFO] `)
	})

	t.Run("should not change the context", func(t *testing.T) {
		ctx := container.Bag{"a": container.Bag{"b": 1}}
		_ = serialize(t, "m", ctx)
		testutil.Equal(t, "ctx", ctx, container.Bag{"a": container.Bag{"b": 1}})
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		s := newSerializer(t, stringify.Id, nil)
		ctx := container.Bag{"a": 1}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 100 {
					if got := s.Serialize(ts, flam.LogInfo, "m", ctx); got != "2026-11-02T09:43:00.123+0100 [INFO] m a=1" {
						t.Errorf("line = %q", got)
					}
				}
			})
		}
		wg.Wait()
	})
}
