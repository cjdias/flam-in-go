package bundle

import (
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// testBundle is a bundle whose Register calls register, when set.
type testBundle struct {
	id       string
	register func(app flam.IApp) error
}

func (b *testBundle) Id() string { return b.id }

func (b *testBundle) Register(app flam.IApp) error {
	if b.register != nil {
		return b.register(app)
	}
	return nil
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ErrorFieldIndex", bundle.ErrorFieldIndex, "index")
	testutil.Equal(t, "ErrorFieldKind", bundle.ErrorFieldKind, "kind")

	sentinels := []error{
		bundle.ErrNilApplication, bundle.ErrNilBundle, bundle.ErrInvalidFunction, bundle.ErrRegistering,
	}
	for i, err := range sentinels {
		for j, other := range sentinels {
			if i != j && errors.Is(err, other) {
				t.Errorf("%q matches %q", err, other)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func Test_Register(t *testing.T) {
	t.Run("should apply the entries in order, with their options", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		got := 0
		if r.QueueProvider(func() int { return 1 }, dig.Name("n")) != r {
			t.Fatal("QueueProvider() did not return the registerer")
		}
		r.QueueProvider(func() int { return 2 }).
			QueueDecorator(func(i int) int { return i + 10 }).
			QueueInvoker(func(i int) { got = i }).
			QueueBundle(&testBundle{id: "b"})

		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(),
			[]string{"provider/1", "provider/0", "decorator/0", "invoker/0", "bundle:b"})
		testutil.Equal(t, "invoked with", got, 12)
	})

	t.Run("should do nothing without entries", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, bundle.NewRegisterer(app).Register())
		testutil.Equal(t, "calls", app.Calls(), []string(nil))
	})

	t.Run("should apply the entries queued after a Register with the next one", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		testutil.NoError(t, r.QueueProvider(func() int { return 1 }).Register())
		testutil.NoError(t, r.QueueInvoker(func(int) {}).Register())
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "invoker/0"})
	})

	t.Run("should fail on a nil application", func(t *testing.T) {
		var typedNil *testutil.App
		for name, app := range map[string]flam.IApp{"nil": nil, "nil pointer": typedNil} {
			err := bundle.NewRegisterer(app).QueueInvoker(func() {}).Register()
			if err == nil {
				t.Fatalf("%s: Register() = nil", name)
			}
			testutil.Error(t, err, bundle.ErrNilApplication,
				"unable to register: nil registerer application", map[string]any{})
		}
	})
}

func Test_RegisterRefusals(t *testing.T) {
	t.Run("should refuse a nil bundle before applying anything", func(t *testing.T) {
		var typedNil *testBundle
		for _, b := range []flam.IBundle{nil, typedNil} {
			app := testutil.NewApp()
			r := bundle.NewRegisterer(app).QueueProvider(func() int { return 1 }).QueueBundle(b)

			for range 2 { // a refused entry stays queued
				testutil.Error(t, r.Register(), bundle.ErrNilBundle,
					"unable to register: nil registerer bundle", map[string]any{"index": 1, "kind": "bundle"})
			}
			testutil.Equal(t, "calls", app.Calls(), []string(nil))
		}
	})

	t.Run("should refuse a value that is not a non-nil function", func(t *testing.T) {
		var nilFunc func() int
		for _, kind := range []string{"provider", "invoker", "decorator"} {
			for _, value := range []any{nil, 1, "f", nilFunc} {
				app := testutil.NewApp()
				r := bundle.NewRegisterer(app).QueueInvoker(func() {})
				switch kind {
				case "provider":
					r.QueueProvider(value)
				case "invoker":
					r.QueueInvoker(value)
				default:
					r.QueueDecorator(value)
				}

				testutil.Error(t, r.Register(), bundle.ErrInvalidFunction,
					"unable to register: invalid registerer function", map[string]any{"index": 1, "kind": kind})
				testutil.Equal(t, "calls", app.Calls(), []string(nil))
			}
		}
	})
}

func Test_RegisterFailures(t *testing.T) {
	t.Run("should wrap the application error and resume after the failed entry", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		fail := true
		app.Fail = func(kind string) error {
			if kind == "invoker" && fail {
				return cause
			}
			return nil
		}
		r := bundle.NewRegisterer(app).
			QueueProvider(func() int { return 1 }).
			QueueInvoker(func(int) {}).
			QueueDecorator(func(i int) int { return i })

		testutil.Error(t, r.Register(), cause, "unable to register: boom", map[string]any{"index": 1, "kind": "invoker"})
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "invoker/0"})

		fail = false
		testutil.NoError(t, r.Register())
		// each entry is applied at most once, the failed one included
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "invoker/0", "decorator/0"})
	})

	t.Run("should not register again a bundle whose Register failed", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).
			QueueBundle(&testBundle{id: "b", register: func(flam.IApp) error { return cause }}).
			QueueInvoker(func() {})

		testutil.Error(t, r.Register(), cause, "unable to register: boom", map[string]any{"index": 0, "kind": "bundle"})
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:b", "invoker/0"})
	})

	t.Run("should wrap the bundle error", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).QueueBundle(&testBundle{id: "b", register: func(flam.IApp) error { return cause }})
		testutil.Error(t, r.Register(), cause, "unable to register: boom", map[string]any{"index": 0, "kind": "bundle"})
	})

	t.Run("should stay usable after an application panic, past the entry", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(kind string) error { panic("boom") }
		r := bundle.NewRegisterer(app).QueueInvoker(func() {})

		testutil.Panics(t, "Register()", func() { _ = r.Register() })
		app.Fail = nil
		testutil.NoError(t, r.QueueProvider(func() int { return 1 }).Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0", "provider/0"})
	})
}

func Test_RegisterTypedNilError(t *testing.T) {
	t.Run("should take a nil error held by a non-nil error as a success", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return (*flam.Error)(nil) }
		r := bundle.NewRegisterer(app).QueueProvider(func() int { return 1 })
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0"})
	})
}

func Test_RegisterReentrancy(t *testing.T) {
	t.Run("should apply the entries queued by an applied entry", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() {
			r.QueueProvider(func() int { return 1 }).QueueInvoker(func(int) {})
		}).QueueBundle(&testBundle{id: "b", register: func(flam.IApp) error {
			r.QueueDecorator(func(i int) int { return i })
			return nil
		}})

		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(),
			[]string{"invoker/0", "bundle:b", "provider/0", "invoker/0", "decorator/0"})
	})

	t.Run("should check the entries queued by an applied entry", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() { r.QueueProvider(func() int { return 1 }).QueueInvoker(nil) })

		testutil.Error(t, r.Register(), bundle.ErrInvalidFunction,
			"unable to register: invalid registerer function", map[string]any{"index": 2, "kind": "invoker"})
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0"})
	})

	t.Run("should fail a Register called from an applied entry", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		var nested error
		r.QueueInvoker(func() { nested = r.Register() })

		testutil.NoError(t, r.Register())
		testutil.Error(t, nested, bundle.ErrRegistering,
			"unable to register: registerer already registering", map[string]any{})
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 20 {
					r.QueueInvoker(func() {})
					if err := r.Register(); err != nil && !errors.Is(err, bundle.ErrRegistering) {
						t.Error(err)
					}
				}
			})
		}
		wg.Wait()
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", len(app.Calls()), 160)
	})
}

// ---------------------------------------------------------------------------
// sentinel errors
// ---------------------------------------------------------------------------

func Test_Sentinels(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		text string
	}{
		{"ErrNilApplication", bundle.ErrNilApplication, "nil registerer application"},
		{"ErrNilBundle", bundle.ErrNilBundle, "nil registerer bundle"},
		{"ErrInvalidFunction", bundle.ErrInvalidFunction, "invalid registerer function"},
		{"ErrRegistering", bundle.ErrRegistering, "registerer already registering"},
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

// ---------------------------------------------------------------------------
// queueing
// ---------------------------------------------------------------------------

// funcBundle is a bundle of a func kind, which a nil value of makes a nil
// bundle that is not a pointer.
type funcBundle func() string

func (b funcBundle) Id() string               { return b() }
func (b funcBundle) Register(flam.IApp) error { return nil }

// valueBundle is a bundle of a struct kind, which is never nil.
type valueBundle struct{ id string }

func (b valueBundle) Id() string               { return b.id }
func (b valueBundle) Register(flam.IApp) error { return nil }

func Test_Queue(t *testing.T) {
	t.Run("should return the registerer from every queue method, for chaining", func(t *testing.T) {
		r := bundle.NewRegisterer(testutil.NewApp())
		for name, got := range map[string]flam.IRegisterer{
			"QueueBundle()":    r.QueueBundle(&testBundle{id: "b"}),
			"QueueProvider()":  r.QueueProvider(func() int { return 1 }),
			"QueueInvoker()":   r.QueueInvoker(func() {}),
			"QueueDecorator()": r.QueueDecorator(func(i int) int { return i }),
		} {
			if got != r {
				t.Errorf("%s did not return the registerer", name)
			}
		}
	})

	t.Run("should not apply anything until Register is called", func(t *testing.T) {
		app := testutil.NewApp()
		ran := false
		bundle.NewRegisterer(app).
			QueueBundle(&testBundle{id: "b"}).
			QueueProvider(func() int { return 1 }).
			QueueInvoker(func() { ran = true }).
			QueueDecorator(func(i int) int { return i })
		testutil.Equal(t, "calls", app.Calls(), []string(nil))
		testutil.Equal(t, "invoker ran", ran, false)
	})

	t.Run("should queue invalid entries without panicking, leaving the refusal to Register", func(t *testing.T) {
		testutil.NotPanics(t, "queueing invalid entries", func() {
			bundle.NewRegisterer(testutil.NewApp()).
				QueueBundle(nil).QueueProvider(nil).QueueInvoker(1).QueueDecorator("f")
		})
	})

	t.Run("should queue on a nil application, leaving the refusal to Register", func(t *testing.T) {
		testutil.NotPanics(t, "NewRegisterer(nil)", func() {
			r := bundle.NewRegisterer(nil)
			if r == nil {
				t.Fatal("NewRegisterer(nil) = nil")
			}
			if r.QueueInvoker(func() {}) != r {
				t.Error("QueueInvoker() did not return the registerer")
			}
		})
	})
}

// ---------------------------------------------------------------------------
// application of the entries
// ---------------------------------------------------------------------------

func Test_RegisterApplication(t *testing.T) {
	t.Run("should apply a bundle with RegisterBundle", func(t *testing.T) {
		app := testutil.NewApp()
		var got flam.IApp
		b := &testBundle{id: "b", register: func(a flam.IApp) error { got = a; return nil }}
		testutil.NoError(t, bundle.NewRegisterer(app).QueueBundle(b).Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:b"})
		testutil.Equal(t, "registered", app.HasBundle("b"), true)
		if got != flam.IApp(app) {
			t.Error("the bundle was not registered on the registerer application")
		}
	})

	t.Run("should apply a bundle of any non-nil kind", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, bundle.NewRegisterer(app).
			QueueBundle(valueBundle{id: "value"}).
			QueueBundle(funcBundle(func() string { return "func" })).
			Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:value", "bundle:func"})
	})

	t.Run("should apply a provider with Provide, making its value resolvable", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, bundle.NewRegisterer(app).QueueProvider(func() string { return "v" }).Register())
		var got string
		testutil.NoError(t, app.Invoke(func(s string) { got = s }))
		testutil.Equal(t, "provided", got, "v")
	})

	t.Run("should apply an invoker with Invoke, calling it once", func(t *testing.T) {
		app := testutil.NewApp()
		calls := 0
		r := bundle.NewRegisterer(app).QueueInvoker(func() { calls++ })
		testutil.NoError(t, r.Register())
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "invoker calls", calls, 1)
	})

	t.Run("should apply a decorator with Decorate, decorating the provided value", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, bundle.NewRegisterer(app).
			QueueProvider(func() string { return "v" }).
			QueueDecorator(func(s string) string { return s + "+d" }).
			Register())
		var got string
		testutil.NoError(t, app.Invoke(func(s string) { got = s }))
		testutil.Equal(t, "decorated", got, "v+d")
	})

	t.Run("should accept any non-nil function, such as a method value or a variadic function", func(t *testing.T) {
		app := testutil.NewApp()
		b := &testBundle{id: "m"}
		testutil.NoError(t, bundle.NewRegisterer(app).
			QueueProvider(b.Id).
			QueueInvoker(func(s string, _ ...int) {}).
			Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "invoker/0"})
	})

	t.Run("should forward the options of every kind, as given", func(t *testing.T) {
		app := testutil.NewApp()
		var provideInfo dig.ProvideInfo
		var invokeInfo dig.InvokeInfo
		var decorateInfo dig.DecorateInfo
		testutil.NoError(t, bundle.NewRegisterer(app).
			QueueProvider(func() int { return 1 }, dig.FillProvideInfo(&provideInfo), dig.Name("n")).
			QueueProvider(func() string { return "s" }).
			QueueDecorator(func(s string) string { return s }, dig.FillDecorateInfo(&decorateInfo)).
			QueueInvoker(func(string) {}, dig.FillInvokeInfo(&invokeInfo)).
			QueueInvoker(func() {}).
			QueueProvider(func() float64 { return 1 }).
			QueueDecorator(func(f float64) float64 { return f }).
			Register())

		testutil.Equal(t, "calls", app.Calls(), []string{
			"provider/2", "provider/0", "decorator/1", "invoker/1", "invoker/0", "provider/0", "decorator/0",
		})
		testutil.Equal(t, "provided outputs", len(provideInfo.Outputs), 1)
		testutil.Equal(t, "provided output", provideInfo.Outputs[0].String(), `int[name = "n"]`)
		testutil.Equal(t, "invoked inputs", len(invokeInfo.Inputs), 1)
		testutil.Equal(t, "decorated outputs", len(decorateInfo.Outputs), 1)
	})

	t.Run("should report the absolute index of an entry, across Register calls", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).QueueProvider(func() int { return 1 }).QueueInvoker(func(int) {})
		testutil.NoError(t, r.Register())

		r.QueueDecorator(func(i int) int { return i }).QueueBundle(nil)
		testutil.Error(t, r.Register(), bundle.ErrNilBundle,
			"unable to register: nil registerer bundle", map[string]any{"index": 3, "kind": "bundle"})
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "invoker/0"})
	})
}

// ---------------------------------------------------------------------------
// refusals
// ---------------------------------------------------------------------------

func Test_RegisterRefusalCases(t *testing.T) {
	var nilFunc func()
	var nilProvider func() int
	var nilPointer *int
	fn := func() {}
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"nil", nil},
		{"an int", 1},
		{"a string", "f"},
		{"a struct", struct{}{}},
		{"a pointer to a function", &fn},
		{"a nil pointer", nilPointer},
		{"a channel", make(chan int)},
		{"a map", map[string]any{}},
		{"a nil func()", nilFunc},
		{"a nil func() int", nilProvider},
	} {
		for _, kind := range []string{"provider", "invoker", "decorator"} {
			t.Run("should refuse "+tc.name+" as a "+kind, func(t *testing.T) {
				app := testutil.NewApp()
				r := bundle.NewRegisterer(app)
				switch kind {
				case "provider":
					r.QueueProvider(tc.value)
				case "invoker":
					r.QueueInvoker(tc.value)
				default:
					r.QueueDecorator(tc.value)
				}
				testutil.Error(t, r.Register(), bundle.ErrInvalidFunction,
					"unable to register: invalid registerer function", map[string]any{"index": 0, "kind": kind})
				testutil.Equal(t, "calls", app.Calls(), []string(nil))
			})
		}
	}

	t.Run("should refuse a nil bundle of a kind other than a pointer", func(t *testing.T) {
		app := testutil.NewApp()
		var nilFuncBundle funcBundle
		testutil.Error(t, bundle.NewRegisterer(app).QueueBundle(nilFuncBundle).Register(), bundle.ErrNilBundle,
			"unable to register: nil registerer bundle", map[string]any{"index": 0, "kind": "bundle"})
		testutil.Equal(t, "calls", app.Calls(), []string(nil))
	})

	t.Run("should check every entry before applying any, the refused one being last", func(t *testing.T) {
		app := testutil.NewApp()
		ran := false
		r := bundle.NewRegisterer(app).
			QueueBundle(&testBundle{id: "b"}).
			QueueProvider(func() int { return 1 }).
			QueueInvoker(func(int) { ran = true }).
			QueueDecorator(func(i int) int { return i }).
			QueueDecorator(nil)
		testutil.Error(t, r.Register(), bundle.ErrInvalidFunction,
			"unable to register: invalid registerer function", map[string]any{"index": 4, "kind": "decorator"})
		testutil.Equal(t, "calls", app.Calls(), []string(nil))
		testutil.Equal(t, "invoker ran", ran, false)
		testutil.Equal(t, "bundle registered", app.HasBundle("b"), false)
	})

	t.Run("should report the first refused entry", func(t *testing.T) {
		r := bundle.NewRegisterer(testutil.NewApp()).
			QueueInvoker(func() {}).
			QueueProvider(1).
			QueueBundle(nil).
			QueueDecorator(nil)
		testutil.Error(t, r.Register(), bundle.ErrInvalidFunction,
			"unable to register: invalid registerer function", map[string]any{"index": 1, "kind": "provider"})
	})

	t.Run("should keep refusing every Register, even once valid entries are queued after the refused one", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).QueueInvoker("f")
		for i := range 3 {
			r.QueueInvoker(func() {})
			testutil.Error(t, r.Register(), bundle.ErrInvalidFunction,
				"unable to register: invalid registerer function", map[string]any{"index": 0, "kind": "invoker"})
			testutil.Equal(t, "calls after Register "+string(rune('1'+i)), app.Calls(), []string(nil))
		}
	})

	t.Run("should not check again the entries already applied", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).QueueInvoker(func() {})
		testutil.NoError(t, r.Register())
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0"})
	})
}

// ---------------------------------------------------------------------------
// nil application
// ---------------------------------------------------------------------------

func Test_RegisterNilApplication(t *testing.T) {
	var typedNil *testutil.App
	for name, app := range map[string]flam.IApp{"a nil application": nil, "a nil pointer application": typedNil} {
		t.Run("should fail on "+name+", without any entry", func(t *testing.T) {
			testutil.Error(t, bundle.NewRegisterer(app).Register(), bundle.ErrNilApplication,
				"unable to register: nil registerer application", map[string]any{})
		})

		t.Run("should fail on "+name+" before checking the entries", func(t *testing.T) {
			testutil.Error(t, bundle.NewRegisterer(app).QueueBundle(nil).QueueInvoker(1).Register(),
				bundle.ErrNilApplication, "unable to register: nil registerer application", map[string]any{})
		})

		t.Run("should fail on "+name+" on every Register", func(t *testing.T) {
			r := bundle.NewRegisterer(app).QueueInvoker(func() {})
			for range 2 {
				testutil.Error(t, r.Register(), bundle.ErrNilApplication,
					"unable to register: nil registerer application", map[string]any{})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// application failures
// ---------------------------------------------------------------------------

// causeError is an error type, to find in the chain with errors.As.
type causeError struct{ text string }

func (e *causeError) Error() string { return e.text }

func Test_RegisterApplicationFailures(t *testing.T) {
	for _, kind := range []string{"bundle", "provider", "invoker", "decorator"} {
		t.Run("should wrap the application error of a "+kind+", with its index and kind", func(t *testing.T) {
			cause := &causeError{text: "boom"}
			app := testutil.NewApp()
			app.Fail = func(k string) error {
				if k == kind {
					return cause
				}
				return nil
			}
			r := bundle.NewRegisterer(app).
				QueueProvider(func() int { return 1 }).
				QueueBundle(&testBundle{id: "b"}).
				QueueInvoker(func(int) {}).
				QueueDecorator(func(i int) int { return i })
			index := map[string]int{"provider": 0, "bundle": 1, "invoker": 2, "decorator": 3}[kind]

			err := r.Register()
			testutil.Error(t, err, cause, "unable to register: boom", map[string]any{"index": index, "kind": kind})
			var target *causeError
			if !errors.As(err, &target) || target != cause {
				t.Errorf("errors.As() did not find the application error")
			}
			testutil.Equal(t, "entries applied", len(app.Calls()), index+1)
		})
	}

	t.Run("should wrap an application error that is itself an *Error, keeping each context", func(t *testing.T) {
		cause := flam.NewError("inner", map[string]any{"inner": true})
		app := testutil.NewApp()
		app.Fail = func(string) error { return cause }
		err := bundle.NewRegisterer(app).QueueInvoker(func() {}).Register()
		testutil.Error(t, err, cause, "unable to register: inner", map[string]any{"index": 0, "kind": "invoker"})
		if errors.Unwrap(err) != error(cause) {
			t.Error("Unwrap() did not return the application error")
		}
		testutil.Context(t, cause, map[string]any{"inner": true})
	})

	t.Run("should wrap the error of the container, such as for an invalid provider", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).QueueProvider(func() {}) // a function, but providing nothing
		err := r.Register()
		if dig.New().Provide(func() {}) == nil {
			t.Fatal("dig accepted a provider providing nothing")
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("error = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{"index": 0, "kind": "provider"})
		if errors.Unwrap(err) == nil || err.Error() != "unable to register: "+errors.Unwrap(err).Error() {
			t.Errorf("Error() = %q, want the container error after the message", err.Error())
		}
	})

	t.Run("should wrap the error returned by an invoker, which ran and is not run again", func(t *testing.T) {
		cause := errors.New("invoker failed")
		app := testutil.NewApp()
		runs := 0
		r := bundle.NewRegisterer(app).QueueInvoker(func() error { runs++; return cause })
		testutil.Error(t, r.Register(), cause, "unable to register: invoker failed",
			map[string]any{"index": 0, "kind": "invoker"})
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "invoker runs", runs, 1)
	})

	t.Run("should count a bundle whose Register failed as registered", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).
			QueueBundle(&testBundle{id: "b", register: func(flam.IApp) error { return errors.New("boom") }})
		_ = r.Register()
		testutil.Equal(t, "registered", app.HasBundle("b"), true)
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:b"})
	})

	t.Run("should not apply the entries after the failed one until the next Register", func(t *testing.T) {
		app := testutil.NewApp()
		fail := true
		app.Fail = func(kind string) error {
			if kind == "provider" && fail {
				fail = false
				return errors.New("boom")
			}
			return nil
		}
		r := bundle.NewRegisterer(app).
			QueueInvoker(func() {}).
			QueueProvider(func() int { return 1 }).
			QueueInvoker(func() {})
		_ = r.Register()
		testutil.Equal(t, "calls after the failure", app.Calls(), []string{"invoker/0", "provider/0"})

		r.QueueDecorator(func(s string) string { return s })
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls after resuming", app.Calls(),
			[]string{"invoker/0", "provider/0", "invoker/0", "decorator/0"})
	})

	t.Run("should resume after successive failures, each entry applied once", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errors.New("boom") }
		r := bundle.NewRegisterer(app).QueueInvoker(func() {}).QueueInvoker(func() {}).QueueInvoker(func() {})
		for i := range 3 {
			err := r.Register()
			var e *flam.Error
			if !errors.As(err, &e) {
				t.Fatalf("Register %d error = %v, want a *flam.Error", i+1, err)
			}
			testutil.Context(t, e, map[string]any{"index": i, "kind": "invoker"})
		}
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0", "invoker/0", "invoker/0"})
	})
}

// ---------------------------------------------------------------------------
// panics
// ---------------------------------------------------------------------------

func Test_RegisterPanics(t *testing.T) {
	t.Run("should let a panic of the application propagate", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { panic("boom") }
		r := bundle.NewRegisterer(app).QueueInvoker(func() {})
		defer func() {
			testutil.Equal(t, "recovered", recover(), any("boom"))
		}()
		_ = r.Register()
		t.Error("Register() did not panic")
	})

	t.Run("should count the entry that panicked as applied, and not apply it again", func(t *testing.T) {
		app := testutil.NewApp()
		runs := 0
		r := bundle.NewRegisterer(app).QueueInvoker(func() { runs++; panic("boom") })
		testutil.Panics(t, "Register()", func() { _ = r.Register() })
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "invoker runs", runs, 1)
	})

	t.Run("should not apply the entries after the one that panicked until the next Register", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).
			QueueBundle(&testBundle{id: "b", register: func(flam.IApp) error { panic("boom") }}).
			QueueInvoker(func() {})
		testutil.Panics(t, "Register()", func() { _ = r.Register() })
		testutil.Equal(t, "calls after the panic", app.Calls(), []string{"bundle:b"})
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls after resuming", app.Calls(), []string{"bundle:b", "invoker/0"})
	})

	t.Run("should release the registerer on a panic, so that queueing and Register do not block nor fail", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app).QueueInvoker(func() { panic("boom") })
		testutil.Panics(t, "Register()", func() { _ = r.Register() })

		done := make(chan error, 1)
		go func() { done <- r.QueueInvoker(func() {}).Register() }()
		select {
		case err := <-done:
			testutil.NoError(t, err) // not ErrRegistering: the panicking call ended
		case <-time.After(2 * time.Second):
			t.Fatal("the registerer stayed locked after a panic")
		}
	})
}

// ---------------------------------------------------------------------------
// re-entrancy and concurrency
// ---------------------------------------------------------------------------

func Test_RegisterQueuedDuringRegister(t *testing.T) {
	t.Run("should apply the entries queued by a bundle it applies, after the entries already queued", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueBundle(&testBundle{id: "outer", register: func(flam.IApp) error {
			r.QueueBundle(&testBundle{id: "inner", register: func(flam.IApp) error {
				r.QueueInvoker(func() {})
				return nil
			}})
			return nil
		}}).QueueProvider(func() int { return 1 })

		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:outer", "provider/0", "bundle:inner", "invoker/0"})
	})

	t.Run("should apply the entries queued by another goroutine while an entry is applied", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() {
			queued := make(chan struct{})
			go func() {
				r.QueueProvider(func() int { return 1 }) // does not wait for the Register call
				close(queued)
			}()
			<-queued
		})

		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0", "provider/0"})
	})

	t.Run("should refuse the invalid entries queued by an applied entry on the next Register too", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() { r.QueueBundle(nil) })
		testutil.Error(t, r.Register(), bundle.ErrNilBundle,
			"unable to register: nil registerer bundle", map[string]any{"index": 1, "kind": "bundle"})
		testutil.Error(t, r.Register(), bundle.ErrNilBundle,
			"unable to register: nil registerer bundle", map[string]any{"index": 1, "kind": "bundle"})
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0"})
	})
}

func Test_RegisterReentrantCalls(t *testing.T) {
	t.Run("should fail a Register called from a bundle it applies", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		var nested error
		r.QueueBundle(&testBundle{id: "b", register: func(flam.IApp) error {
			nested = r.Register()
			return nil
		}})
		testutil.NoError(t, r.Register())
		testutil.Error(t, nested, bundle.ErrRegistering,
			"unable to register: registerer already registering", map[string]any{})
	})

	t.Run("should wrap the ErrRegistering returned by an invoker as the error of that invoker", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() error { return r.Register() })
		testutil.Error(t, r.Register(), bundle.ErrRegistering,
			"unable to register: unable to register: registerer already registering",
			map[string]any{"index": 0, "kind": "invoker"})
	})

	t.Run("should not apply anything from the failed nested Register", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() {
			r.QueueProvider(func() int { return 1 })
			_ = r.Register()
			if calls := app.Calls(); len(calls) != 1 {
				t.Errorf("calls during the nested Register = %v", calls)
			}
		})
		testutil.NoError(t, r.Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0", "provider/0"})
	})

	t.Run("should fail a Register called by another goroutine instead of waiting", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		var concurrent error
		r.QueueInvoker(func() {
			done := make(chan struct{})
			go func() {
				concurrent = r.Register()
				close(done)
			}()
			<-done // would never return if the Register call waited for this one
		})
		testutil.NoError(t, r.Register())
		testutil.Error(t, concurrent, bundle.ErrRegistering,
			"unable to register: registerer already registering", map[string]any{})
	})

	t.Run("should accept a Register once the previous one returned", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		r.QueueInvoker(func() { _ = r.Register() })
		testutil.NoError(t, r.Register())
		testutil.NoError(t, r.QueueInvoker(func() {}).Register())
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0", "invoker/0"})
	})

	t.Run("should not share the registering state between registerers", func(t *testing.T) {
		app := testutil.NewApp()
		outer, inner := bundle.NewRegisterer(app), bundle.NewRegisterer(app)
		var nested error
		outer.QueueInvoker(func() { nested = inner.QueueProvider(func() int { return 1 }).Register() })
		testutil.NoError(t, outer.Register())
		testutil.NoError(t, nested)
		testutil.Equal(t, "calls", app.Calls(), []string{"invoker/0", "provider/0"})
	})
}

func Test_RegisterConcurrency(t *testing.T) {
	t.Run("should apply each entry queued concurrently exactly once", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		var runs sync.Map
		var wg sync.WaitGroup
		for g := range 8 {
			wg.Go(func() {
				for i := range 25 {
					key := [2]int{g, i}
					r.QueueInvoker(func() {
						if _, loaded := runs.LoadOrStore(key, true); loaded {
							t.Errorf("invoker %v ran twice", key)
						}
					})
					if err := r.Register(); err != nil && !errors.Is(err, bundle.ErrRegistering) {
						t.Error(err)
					}
				}
			})
		}
		wg.Wait()
		testutil.NoError(t, r.Register())
		count := 0
		runs.Range(func(any, any) bool { count++; return true })
		testutil.Equal(t, "invokers run", count, 200)
	})

	t.Run("should keep the queue order of each goroutine", func(t *testing.T) {
		app := testutil.NewApp()
		r := bundle.NewRegisterer(app)
		var mu sync.Mutex
		order := map[int][]int{}
		var wg sync.WaitGroup
		for g := range 4 {
			wg.Go(func() {
				for i := range 50 {
					r.QueueInvoker(func() {
						mu.Lock()
						order[g] = append(order[g], i)
						mu.Unlock()
					})
				}
			})
		}
		wg.Wait()
		testutil.NoError(t, r.Register())
		for g := range 4 {
			for i, got := range order[g] {
				if got != i {
					t.Fatalf("goroutine %d: entry %d applied at position %d", g, got, i)
				}
			}
			testutil.Equal(t, "entries of a goroutine", len(order[g]), 50)
		}
	})
}
