package log

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// fakeSerializer is a flam.ILogSerializer failing to close with closeErr.
type fakeSerializer struct {
	id       string
	closeErr error
}

func (s *fakeSerializer) Id() string   { return s.id }
func (s *fakeSerializer) Close() error { return s.closeErr }
func (s *fakeSerializer) Serialize(_ time.Time, _ flam.LogLevel, message string, _ flam.IBag) string {
	return message
}

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", log.NewBundle().Id(), "flam.log")
		testutil.Equal(t, "Id()", serializers.NewBundle().Id(), "flam.log.serializers")
		testutil.Equal(t, "Id()", streams.NewBundle().Id(), "flam.log.streams")
	})

	t.Run("should register its providers", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, log.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"provider/0", "provider/0", "provider/1", "provider/1", "provider/0", "provider/0",
		})
	})

	t.Run("should register the serializer drivers", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, serializers.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.log.serializers.stringify", "provider/1",
			"bundle:flam.log.serializers.json", "provider/1",
		})
	})

	t.Run("should register the stream drivers", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, streams.NewBundle().Register(app))
		want := []string{
			"bundle:flam.log.streams.console", "provider/1",
			"bundle:flam.log.streams.file", "provider/1",
			"bundle:flam.log.streams.rotating_file", "provider/1",
		}
		if runtime.GOOS != "windows" && runtime.GOOS != "plan9" {
			want = append(want, "bundle:flam.log.streams.syslog", "provider/1")
		}
		want = append(want, "bundle:flam.log.streams.otlp", "provider/1")
		testutil.Equal(t, "calls", app.Calls(), want)
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := log.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})

	t.Run("should provide the factories of the configuration, with the creators of the groups", func(t *testing.T) {
		bag := defaults(t, nil, map[string][]string{"a": nil})
		testutil.NoError(t, bag.Set(log.SerializerConfigPath+".s", container.Bag{"driver": "fake"}))
		a := newLogApp(t, bag, nil, func(app *testutil.App) {
			testutil.NoError(t, app.Provide(func() flam.ILogSerializerCreator {
				return &serializerCreator{}
			}, dig.Group(log.SerializerCreatorGroup)))
		})
		testutil.Equal(t, "streams", a.factory(t).Available(), []string{"a"})
		testutil.NoError(t, a.Invoke(func(f flam.ILogSerializerFactory) {
			testutil.Equal(t, "serializers", f.Available(), []string{"s"})
			s, err := f.Get("s")
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", s.Id(), "s")
		}))
	})
}

// serializerCreator is a flam.ILogSerializerCreator of fakeSerializer
// values, for the "fake" driver, failing to close with the "close" flag.
type serializerCreator struct{}

func (c *serializerCreator) Accept(bag flam.IBag) bool { return bag.String("driver") == "fake" }

func (c *serializerCreator) Create(bag flam.IBag) (flam.ILogSerializer, error) {
	s := &fakeSerializer{id: bag.String(factory.ConfigFieldId)}
	if bag.Bool("close") {
		s.closeErr = errBoom
	}
	return s, nil
}

func Test_Bundle_Boot(t *testing.T) {
	t.Run("should create the configured streams, then start the flusher", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "1s", map[string][]string{"b": nil, "a": nil}), triggers)
		testutil.NoError(t, a.boot())
		testutil.Equal(t, "stored", a.factory(t).Stored(), []string{"a", "b"})
		testutil.Equal(t, "created", len(triggers.all()), 1)
		testutil.Equal(t, "observing", a.observing(), true)
	})

	t.Run("should create no stream when not booting them, but start the flusher", func(t *testing.T) {
		bag := defaults(t, nil, map[string][]string{"a": nil})
		testutil.NoError(t, bag.Set(log.BootConfigPath, false))
		a := newLogApp(t, bag, nil)
		testutil.NoError(t, a.boot())
		testutil.Equal(t, "stored", a.factory(t).Stored(), []string{})
		testutil.Equal(t, "observing", a.observing(), true)
	})

	t.Run("should stop at the first stream failing to be created", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil, "b": {"fail"}, "c": nil}), nil)
		err := a.boot()
		if !errors.Is(err, errBoom) {
			t.Fatalf("Boot() = %v, want boom", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("Boot() = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{log.ErrorFieldID: "b"})
		testutil.Equal(t, "prefix", err.Error()[:len("unable to boot log stream: ")], "unable to boot log stream: ")
		testutil.Equal(t, "stored", a.factory(t).Stored(), []string{"a"})
		testutil.Equal(t, "observing", a.observing(), false)
	})

	t.Run("should fail for a stream no creator accepts", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(log.StreamConfigPath+".a", container.Bag{"driver": "other"}))
		a := newLogApp(t, bag, nil)
		if err := a.boot(); !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Boot() = %v, want ErrNoCreator", err)
		}
	})

	t.Run("should fail when the bundle is not registered", func(t *testing.T) {
		if err := log.NewBundle().(flam.IBootableBundle).Boot(testutil.NewApp()); err == nil {
			t.Fatal("Boot() = nil, want an error")
		}
	})
}

func Test_Bundle_Close(t *testing.T) {
	t.Run("should flush, then close the streams, then the serializers", func(t *testing.T) {
		bag := defaults(t, nil, map[string][]string{"a": nil})
		testutil.NoError(t, bag.Set(log.SerializerConfigPath+".s", container.Bag{"driver": "fake"}))
		var serializers flam.ILogSerializerFactory
		a := newLogApp(t, bag, nil, func(app *testutil.App) {
			testutil.NoError(t, app.Provide(func() flam.ILogSerializerCreator {
				return &serializerCreator{}
			}, dig.Group(log.SerializerCreatorGroup)))
		})
		testutil.NoError(t, a.Invoke(func(f flam.ILogSerializerFactory) { serializers = f }))
		_, err := serializers.Get("s")
		testutil.NoError(t, err)
		testutil.NoError(t, a.boot())
		a.logger(t).BroadcastInfo("m")

		testutil.NoError(t, a.close())
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{"m"})
		testutil.Equal(t, "closed", a.streams.get("a").isClosed(), true)
		testutil.Equal(t, "streams", a.factory(t).Stored(), []string(nil))
		testutil.Equal(t, "serializers", serializers.Stored(), []string(nil))
	})

	t.Run("should close everything, joining the errors", func(t *testing.T) {
		bag := defaults(t, nil, map[string][]string{"a": {"write", "close"}})
		testutil.NoError(t, bag.Set(log.SerializerConfigPath+".s", container.Bag{"driver": "fake", "close": true}))
		var serializers flam.ILogSerializerFactory
		a := newLogApp(t, bag, nil, func(app *testutil.App) {
			testutil.NoError(t, app.Provide(func() flam.ILogSerializerCreator {
				return &serializerCreator{}
			}, dig.Group(log.SerializerCreatorGroup)))
		})
		testutil.NoError(t, a.Invoke(func(f flam.ILogSerializerFactory) { serializers = f }))
		_, err := serializers.Get("s")
		testutil.NoError(t, err)
		testutil.NoError(t, a.boot())
		a.logger(t).BroadcastInfo("m")

		errs := joined(t, a.close())
		testutil.Equal(t, "errors", len(errs), 3)
		for _, e := range errs {
			if !errors.Is(e, errBoom) {
				t.Errorf("error = %v, want boom", e)
			}
		}
		testutil.Equal(t, "flush", errs[0].Error(), "unable to flush log on close: unable to flush log entries to stream: boom")
		testutil.Equal(t, "serializers", serializers.Stored(), []string(nil))
	})

	t.Run("should create, flush and close what was never used", func(t *testing.T) {
		a := newLogApp(t, container.Bag{}, nil)
		testutil.NoError(t, a.close())
	})

	t.Run("should fail when the bundle is not registered", func(t *testing.T) {
		if err := log.NewBundle().(flam.IClosableBundle).Close(testutil.NewApp()); err == nil {
			t.Fatal("Close() = nil, want an error")
		}
	})
}
