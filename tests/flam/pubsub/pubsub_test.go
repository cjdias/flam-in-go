package pubsub

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/pubsub"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

type PubSub = pubsub.PubSub[string, int]

// failure is an error reported to an error callback.
type failure struct {
	topic string
	msg   int
	err   error
}

// recorder records the messages handled and the failures reported by the
// subscriptions using its callbacks.
type recorder struct {
	mu       sync.Mutex
	handled  []int
	failures []failure
}

func (r *recorder) handle(_ context.Context, _ string, msg int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handled = append(r.handled, msg)
	return nil
}

func (r *recorder) onError(topic string, msg int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures = append(r.failures, failure{topic, msg, err})
}

// typedNilError is an error type whose nil pointer a handler may return.
type typedNilError struct{}

func (*typedNilError) Error() string { return "typed nil" }

// waitFor returns the next value received from ch, failing the test, and
// stopping it, when none is received within two seconds.
func waitFor[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
	var zero T
	return zero
}

// notDone fails the test when ch is already closed or holds a value.
func notDone[T any](t *testing.T, ch <-chan T, what string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("%s happened too early", what)
	default:
	}
}

// eventually fails the test, and stops it, unless cond becomes true
// within two seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		runtime.Gosched()
	}
}

// switchCtx is a context whose Err is set by the test, to make a message
// context done between Publish and its handling deterministically.
type switchCtx struct {
	context.Context
	err atomic.Pointer[error]
}

func (c *switchCtx) Err() error {
	if err := c.err.Load(); err != nil {
		return *err
	}
	return nil
}

func (c *switchCtx) set(err error) { c.err.Store(&err) }

// ctxKey is the key of a context value given to Publish.
type ctxKey struct{}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "TopicSeparator", pubsub.TopicSeparator, ".")
	testutil.Equal(t, "ErrorFieldID", pubsub.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldPattern", pubsub.ErrorFieldPattern, "pattern")
	testutil.Equal(t, "ErrorFieldTopic", pubsub.ErrorFieldTopic, "topic")
	testutil.Equal(t, "ErrorFieldPanic", pubsub.ErrorFieldPanic, "panic")
	testutil.Equal(t, "ErrorFieldStack", pubsub.ErrorFieldStack, "stack")

	sentinels := []error{
		pubsub.ErrClosed, pubsub.ErrInvalidPattern, pubsub.ErrInvalidTopic,
		pubsub.ErrNilHandler, pubsub.ErrDuplicateID, pubsub.ErrUnknownID,
		pubsub.ErrInvalidID, pubsub.ErrHandlerPanic, pubsub.ErrHandlerExit,
		pubsub.ErrCloseFromHandler,
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
		{"ErrClosed", pubsub.ErrClosed, "pubsub closed"},
		{"ErrInvalidPattern", pubsub.ErrInvalidPattern, "invalid pubsub pattern"},
		{"ErrInvalidTopic", pubsub.ErrInvalidTopic, "invalid pubsub topic"},
		{"ErrNilHandler", pubsub.ErrNilHandler, "nil pubsub handler"},
		{"ErrDuplicateID", pubsub.ErrDuplicateID, "pubsub subscription id already in use"},
		{"ErrUnknownID", pubsub.ErrUnknownID, "unknown pubsub subscription id"},
		{"ErrInvalidID", pubsub.ErrInvalidID, "invalid pubsub subscription id"},
		{"ErrHandlerPanic", pubsub.ErrHandlerPanic, "pubsub handler panicked"},
		{"ErrHandlerExit", pubsub.ErrHandlerExit, "pubsub handler exited"},
		{"ErrCloseFromHandler", pubsub.ErrCloseFromHandler, "pubsub closed from one of its handlers"},
	} {
		t.Run("should declare "+tc.name+" with its message", func(t *testing.T) {
			testutil.Equal(t, tc.name+".Error()", tc.err.Error(), tc.text)
		})
	}
}

func Test_Contract(t *testing.T) {
	t.Run("should implement IPubSub for any id and data types", func(t *testing.T) {
		var _ flam.IPubSub[string, int] = (*pubsub.PubSub[string, int])(nil)
		var _ flam.IPubSub[any, any] = (*pubsub.PubSub[any, any])(nil)
		var _ flam.IPubSub[struct{ a, b int }, []byte] = (*pubsub.PubSub[struct{ a, b int }, []byte])(nil)
	})

	t.Run("should return a new open, empty *PubSub from New", func(t *testing.T) {
		first, second := pubsub.New[string, int](), pubsub.New[string, int]()
		if first == second {
			t.Fatal("New() returned the same PubSub twice")
		}
		testutil.Error(t, first.Unsubscribe("s"), pubsub.ErrUnknownID,
			"unable to unsubscribe: unknown pubsub subscription id", map[string]any{"id": "s"})
		testutil.NoError(t, first.Publish(context.Background(), "a", 1))
		r := &recorder{}
		testutil.NoError(t, first.Subscribe("s", "a", r.handle, nil))
		testutil.NoError(t, first.Publish(context.Background(), "a", 1))
		testutil.NoError(t, first.Close())
		testutil.NoError(t, second.Close())
		testutil.Equal(t, "handled", r.handled, []int{1})
	})

	t.Run("should make the zero value an empty, open PubSub ready to use", func(t *testing.T) {
		var p PubSub
		testutil.Error(t, p.Unsubscribe("s"), pubsub.ErrUnknownID,
			"unable to unsubscribe: unknown pubsub subscription id", map[string]any{"id": "s"})
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 2))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{2})
	})

	t.Run("should close a zero value never used", func(t *testing.T) {
		var p PubSub
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Publish(context.Background(), "a", 1), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
	})
}

// ---------------------------------------------------------------------------
// topics and patterns
// ---------------------------------------------------------------------------

func Test_Patterns(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pattern string
		matches []string
		misses  []string
	}{
		{"should match a literal pattern to the same topic only", "a",
			[]string{"a"}, []string{"b", "a.b", "b.a", "aa"}},
		{"should compare segments exactly, case and spaces included", "a",
			[]string{"a"}, []string{"A", " a", "a "}},
		{"should match a literal pattern segment by segment", "a.b",
			[]string{"a.b"}, []string{"a", "a.c", "b.b", "a.b.c", "b.a"}},
		{"should match * to exactly one segment", "*",
			[]string{"a", "b", "ab", " "}, []string{"a.b", "a.b.c"}},
		{"should match * in the middle to exactly one segment", "a.*.c",
			[]string{"a.b.c", "a.x.c"}, []string{"a.c", "a.b.d", "a.b.c.d", "a.b.x.c"}},
		{"should match * as first or last segment", "*.b.*",
			[]string{"a.b.c", "x.b.y"}, []string{"a.b", "b.c", "a.c.c", "a.b.c.d"}},
		{"should match each * to its own segment", "*.*",
			[]string{"a.b", "x.y"}, []string{"a", "a.b.c"}},
		{"should match the documented orders.*.created example", "orders.*.created",
			[]string{"orders.eu.created", "orders.us.created"},
			[]string{"orders.created", "orders.eu.us.created", "orders.eu.deleted"}},
		{"should match a trailing ** to one or more segments, but not none, as documented", "orders.**",
			[]string{"orders.eu", "orders.eu.created", "orders.a.b.c.d"},
			[]string{"orders", "order.eu", "eu.orders", "eu.orders.created"}},
		{"should match *.** to two or more segments", "*.**",
			[]string{"a.b", "x.y.z"}, []string{"a"}},
		{"should match a pattern mixing literal, * and ** segments", "a.*.**",
			[]string{"a.b.c", "a.b.c.d"}, []string{"a", "a.b", "b.b.c"}},
		{"should match ** alone to every topic", "**",
			[]string{"a", "a.b", "a.b.c"}, nil},
		{"should split on TopicSeparator only", "*",
			[]string{"a/b", "a-b", "a:b", "a,b", "a_b"}, []string{"a.b"}},
		{"should match segments of any other character", "é.a b.1",
			[]string{"é.a b.1"}, []string{"e.a b.1", "é.ab.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &PubSub{}
			r := &recorder{}
			testutil.NoError(t, p.Subscribe("s", tc.pattern, r.handle, nil))
			for i, topic := range tc.matches {
				testutil.NoError(t, p.Publish(context.Background(), topic, i))
			}
			for _, topic := range tc.misses {
				testutil.NoError(t, p.Publish(context.Background(), topic, -1))
			}
			testutil.NoError(t, p.Close())

			want := []int(nil)
			for i := range tc.matches {
				want = append(want, i)
			}
			testutil.Equal(t, "handled for "+tc.pattern, r.handled, want)
		})
	}

	t.Run("should use . as TopicSeparator", func(t *testing.T) {
		testutil.Equal(t, "TopicSeparator", pubsub.TopicSeparator, ".")
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a"+pubsub.TopicSeparator+"*", r.handle, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a"+pubsub.TopicSeparator+"b", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{1})
	})

	for _, tc := range []struct {
		name    string
		pattern string
	}{
		{"should refuse an empty pattern", ""},
		{"should refuse a pattern made of a separator", "."},
		{"should refuse a pattern with an empty last segment", "a."},
		{"should refuse a pattern with an empty first segment", ".a"},
		{"should refuse a pattern with an empty middle segment", "a..b"},
		{"should refuse a pattern with empty segments around a wildcard", ".*."},
		{"should refuse a segment ending with *", "a*"},
		{"should refuse a segment starting with *", "*a"},
		{"should refuse a segment with * inside", "a*b"},
		{"should refuse a last segment mixing * and other characters", "a.b*"},
		{"should refuse a segment mixing ** and other characters", "a.**b"},
		{"should refuse a segment of three *", "***"},
		{"should refuse ** as first of several segments", "**.a"},
		{"should refuse ** as a middle segment", "a.**.b"},
		{"should refuse ** twice", "**.**"},
		{"should refuse ** before *", "a.**.*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &PubSub{}
			testutil.Error(t, p.Subscribe("s", tc.pattern, (&recorder{}).handle, nil), pubsub.ErrInvalidPattern,
				"unable to subscribe: invalid pubsub pattern", map[string]any{"id": "s", "pattern": tc.pattern})
			testutil.Error(t, p.Unsubscribe("s"), pubsub.ErrUnknownID,
				"unable to unsubscribe: unknown pubsub subscription id", map[string]any{"id": "s"})
		})
	}

	for _, tc := range []struct {
		name  string
		topic string
	}{
		{"should refuse an empty topic", ""},
		{"should refuse a topic made of a separator", "."},
		{"should refuse a topic with an empty last segment", "a."},
		{"should refuse a topic with an empty first segment", ".a"},
		{"should refuse a topic with an empty middle segment", "a..b"},
		{"should refuse a * topic", "*"},
		{"should refuse a ** topic", "**"},
		{"should refuse a topic with a * segment", "a.*"},
		{"should refuse a topic with a ** segment", "a.**"},
		{"should refuse a topic segment ending with *", "a*"},
		{"should refuse a topic segment starting with *", "*a"},
		{"should refuse a topic segment with * inside", "a.b*c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &PubSub{}
			r := &recorder{}
			testutil.NoError(t, p.Subscribe("s", "**", r.handle, nil))
			testutil.Error(t, p.Publish(context.Background(), tc.topic, 0), pubsub.ErrInvalidTopic,
				"unable to publish: invalid pubsub topic", map[string]any{"topic": tc.topic})
			testutil.NoError(t, p.Close())
			testutil.Equal(t, "handled", r.handled, []int(nil))
		})
	}

	t.Run("should accept topics of non-empty segments without *", func(t *testing.T) {
		p := &PubSub{}
		for _, topic := range []string{"a", "a.b.c", " ", "a b.c", "é", "a/b", "a-b_c:d"} {
			testutil.NoError(t, p.Publish(context.Background(), topic, 0))
		}
		testutil.NoError(t, p.Close())
	})
}

// ---------------------------------------------------------------------------
// Subscribe, Unsubscribe
// ---------------------------------------------------------------------------

func Test_Subscribe(t *testing.T) {
	t.Run("should fail on a nil handler", func(t *testing.T) {
		testutil.Error(t, (&PubSub{}).Subscribe("s", "a", nil, nil), pubsub.ErrNilHandler,
			"unable to subscribe: nil pubsub handler", map[string]any{"id": "s", "pattern": "a"})
	})

	t.Run("should fail on an id that cannot be a map key", func(t *testing.T) {
		p := &pubsub.PubSub[any, int]{}
		handler := func(context.Context, string, int) error { return nil }
		testutil.Error(t, p.Subscribe([]int{1}, "a", handler, nil), pubsub.ErrInvalidID,
			"unable to subscribe: invalid pubsub subscription id", map[string]any{"id": []int{1}, "pattern": "a"})
		testutil.Error(t, p.Unsubscribe([]int{1}), pubsub.ErrInvalidID,
			"unable to unsubscribe: invalid pubsub subscription id", map[string]any{"id": []int{1}})
		testutil.NoError(t, p.Subscribe(nil, "a", handler, nil))
		testutil.NoError(t, p.Close())
	})

	t.Run("should fail on an id in use", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, nil))
		testutil.Error(t, p.Subscribe("s", "b", r.handle, nil), pubsub.ErrDuplicateID,
			"unable to subscribe: pubsub subscription id already in use", map[string]any{"id": "s", "pattern": "b"})
		testutil.NoError(t, p.Close())
	})

	t.Run("should check the arguments before the PubSub state", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Subscribe("s", "", (&recorder{}).handle, nil), pubsub.ErrInvalidPattern,
			"unable to subscribe: invalid pubsub pattern", map[string]any{"id": "s", "pattern": ""})
	})
}

func Test_Unsubscribe(t *testing.T) {
	t.Run("should stop the delivery after handling the queued messages", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		release := make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			<-release
			return r.handle(ctx, topic, msg)
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Publish(context.Background(), "a", 2))
		testutil.NoError(t, p.Unsubscribe("s"))
		testutil.NoError(t, p.Publish(context.Background(), "a", 3))
		close(release)
		testutil.NoError(t, p.Close())

		testutil.Equal(t, "handled", r.handled, []int{1, 2})
	})

	t.Run("should let a handler unsubscribe its own subscription", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		done := make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			defer close(done)
			return p.Unsubscribe("s")
		}, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		<-done
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should fail on an unknown id", func(t *testing.T) {
		testutil.Error(t, (&PubSub{}).Unsubscribe("s"), pubsub.ErrUnknownID,
			"unable to unsubscribe: unknown pubsub subscription id", map[string]any{"id": "s"})
	})
}

func Test_SubscribeChecks(t *testing.T) {
	handler := func(context.Context, string, int) error { return nil }

	t.Run("should check the pattern before the handler and the id", func(t *testing.T) {
		p := &pubsub.PubSub[any, int]{}
		testutil.Error(t, p.Subscribe([]int{1}, "", nil, nil), pubsub.ErrInvalidPattern,
			"unable to subscribe: invalid pubsub pattern", map[string]any{"id": []int{1}, "pattern": ""})
	})

	t.Run("should check the handler before the id", func(t *testing.T) {
		p := &pubsub.PubSub[any, int]{}
		testutil.Error(t, p.Subscribe([]int{1}, "a", nil, nil), pubsub.ErrNilHandler,
			"unable to subscribe: nil pubsub handler", map[string]any{"id": []int{1}, "pattern": "a"})
	})

	t.Run("should check the id before the PubSub state", func(t *testing.T) {
		p := &pubsub.PubSub[any, int]{}
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Subscribe([]int{1}, "a", handler, nil), pubsub.ErrInvalidID,
			"unable to subscribe: invalid pubsub subscription id", map[string]any{"id": []int{1}, "pattern": "a"})
	})

	t.Run("should check the handler before the PubSub state", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Subscribe("s", "a", nil, nil), pubsub.ErrNilHandler,
			"unable to subscribe: nil pubsub handler", map[string]any{"id": "s", "pattern": "a"})
	})

	t.Run("should fail with ErrClosed on an id used before Close", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Subscribe("s", "a", handler, nil))
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Subscribe("s", "a", handler, nil), pubsub.ErrClosed,
			"unable to subscribe: pubsub closed", map[string]any{"id": "s", "pattern": "a"})
	})

	t.Run("should accept a nil error callback", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{1})
	})

	t.Run("should keep the subscription using an id when refusing it again", func(t *testing.T) {
		p := &PubSub{}
		first, second := &recorder{}, &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", first.handle, nil))
		testutil.Error(t, p.Subscribe("s", "b", second.handle, nil), pubsub.ErrDuplicateID,
			"unable to subscribe: pubsub subscription id already in use", map[string]any{"id": "s", "pattern": "b"})
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Publish(context.Background(), "b", 2))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "first handled", first.handled, []int{1})
		testutil.Equal(t, "second handled", second.handled, []int(nil))
	})

	t.Run("should fail on an id in use even with the same pattern", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Subscribe("s", "a", handler, nil))
		testutil.Error(t, p.Subscribe("s", "a", handler, nil), pubsub.ErrDuplicateID,
			"unable to subscribe: pubsub subscription id already in use", map[string]any{"id": "s", "pattern": "a"})
		testutil.NoError(t, p.Close())
	})

	t.Run("should let an id be used again once unsubscribed", func(t *testing.T) {
		p := &PubSub{}
		first, second := &recorder{}, &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", first.handle, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Unsubscribe("s"))
		testutil.NoError(t, p.Subscribe("s", "b", second.handle, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 2))
		testutil.NoError(t, p.Publish(context.Background(), "b", 3))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "first handled", first.handled, []int{1})
		testutil.Equal(t, "second handled", second.handled, []int{3})
	})

	t.Run("should deliver to several subscriptions with the same pattern", func(t *testing.T) {
		p := &PubSub{}
		a, b := &recorder{}, &recorder{}
		testutil.NoError(t, p.Subscribe("a", "t", a.handle, nil))
		testutil.NoError(t, p.Subscribe("b", "t", b.handle, nil))
		testutil.NoError(t, p.Publish(context.Background(), "t", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "a handled", a.handled, []int{1})
		testutil.Equal(t, "b handled", b.handled, []int{1})
	})
}

func Test_IDs(t *testing.T) {
	handler := func(context.Context, string, int) error { return nil }

	for _, tc := range []struct {
		name string
		id   any
	}{
		{"should refuse a slice id", []int{1}},
		{"should refuse a nil slice id", []int(nil)},
		{"should refuse a map id", map[string]int{"a": 1}},
		{"should refuse a struct id holding a slice", struct{ s []int }{[]int{1}}},
		{"should refuse a struct id holding an interface holding a slice", struct{ v any }{[]int{1}}},
		{"should refuse an array id holding an interface holding a map", [1]any{map[int]int{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &pubsub.PubSub[any, int]{}
			testutil.Error(t, p.Subscribe(tc.id, "a", handler, nil), pubsub.ErrInvalidID,
				"unable to subscribe: invalid pubsub subscription id", map[string]any{"id": tc.id, "pattern": "a"})
			testutil.Error(t, p.Unsubscribe(tc.id), pubsub.ErrInvalidID,
				"unable to unsubscribe: invalid pubsub subscription id", map[string]any{"id": tc.id})
			testutil.NoError(t, p.Close())
		})
	}

	t.Run("should refuse a function id", func(t *testing.T) {
		// a function is only deeply equal to nil, so the context is
		// checked by key
		p := &pubsub.PubSub[any, int]{}
		for _, err := range []error{p.Subscribe(handler, "a", handler, nil), p.Unsubscribe(handler)} {
			if !errors.Is(err, pubsub.ErrInvalidID) {
				t.Fatalf("error = %v, want it to wrap %v", err, pubsub.ErrInvalidID)
			}
			var e *flam.Error
			if !errors.As(err, &e) {
				t.Fatalf("error = %T, want a *flam.Error", err)
			}
			if _, ok := e.Context()["id"]; !ok {
				t.Errorf("Context() = %v, want an id field", e.Context())
			}
		}
		testutil.NoError(t, p.Close())
	})

	t.Run("should check the id before the PubSub state on Unsubscribe", func(t *testing.T) {
		p := &pubsub.PubSub[any, int]{}
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Unsubscribe([]int{1}), pubsub.ErrInvalidID,
			"unable to unsubscribe: invalid pubsub subscription id", map[string]any{"id": []int{1}})
	})

	ch := make(chan int)
	n := 1
	for _, tc := range []struct {
		name string
		id   any
	}{
		{"should accept a nil id", nil},
		{"should accept a zero id", 0},
		{"should accept an empty string id", ""},
		{"should accept a comparable struct id", struct{ a, b int }{1, 2}},
		{"should accept an array id", [2]string{"a", "b"}},
		{"should accept a pointer id", &n},
		{"should accept a channel id", ch},
		{"should accept a struct id holding an interface holding a comparable value", struct{ v any }{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &pubsub.PubSub[any, int]{}
			got := make(chan int, 1)
			testutil.NoError(t, p.Subscribe(tc.id, "a", func(_ context.Context, _ string, msg int) error {
				got <- msg
				return nil
			}, nil))
			testutil.Error(t, p.Subscribe(tc.id, "a", handler, nil), pubsub.ErrDuplicateID,
				"unable to subscribe: pubsub subscription id already in use", map[string]any{"id": tc.id, "pattern": "a"})
			testutil.NoError(t, p.Publish(context.Background(), "a", 1))
			testutil.Equal(t, "message", waitFor(t, got, "the message"), 1)
			testutil.NoError(t, p.Unsubscribe(tc.id))
			testutil.NoError(t, p.Close())
		})
	}

	t.Run("should tell ids apart by value, as map keys do", func(t *testing.T) {
		p := &pubsub.PubSub[any, int]{}
		testutil.NoError(t, p.Subscribe(1, "a", handler, nil))
		testutil.NoError(t, p.Subscribe(int64(1), "a", handler, nil))
		testutil.NoError(t, p.Subscribe("1", "a", handler, nil))
		testutil.NoError(t, p.Unsubscribe(int64(1)))
		testutil.Error(t, p.Unsubscribe(int64(1)), pubsub.ErrUnknownID,
			"unable to unsubscribe: unknown pubsub subscription id", map[string]any{"id": int64(1)})
		testutil.NoError(t, p.Unsubscribe(1))
		testutil.NoError(t, p.Unsubscribe("1"))
		testutil.NoError(t, p.Close())
	})
}

func Test_UnsubscribeMore(t *testing.T) {
	t.Run("should fail on an id already unsubscribed", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Subscribe("s", "a", (&recorder{}).handle, nil))
		testutil.NoError(t, p.Unsubscribe("s"))
		testutil.Error(t, p.Unsubscribe("s"), pubsub.ErrUnknownID,
			"unable to unsubscribe: unknown pubsub subscription id", map[string]any{"id": "s"})
		testutil.NoError(t, p.Close())
	})

	t.Run("should fail with ErrClosed, not ErrUnknownID, once closed", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Unsubscribe("never"), pubsub.ErrClosed,
			"unable to unsubscribe: pubsub closed", map[string]any{"id": "never"})
	})

	t.Run("should leave the other subscriptions alone", func(t *testing.T) {
		p := &PubSub{}
		a, b := &recorder{}, &recorder{}
		testutil.NoError(t, p.Subscribe("a", "t", a.handle, nil))
		testutil.NoError(t, p.Subscribe("b", "t", b.handle, nil))
		testutil.NoError(t, p.Unsubscribe("a"))
		testutil.NoError(t, p.Publish(context.Background(), "t", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "a handled", a.handled, []int(nil))
		testutil.Equal(t, "b handled", b.handled, []int{1})
	})

	t.Run("should not wait for the handler", func(t *testing.T) {
		p := &PubSub{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			close(started)
			<-release
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		waitFor(t, started, "the handler")

		returned := make(chan error, 1)
		go func() { returned <- p.Unsubscribe("s") }()
		testutil.NoError(t, waitFor(t, returned, "Unsubscribe"))
		close(release)
		testutil.NoError(t, p.Close())
	})

	t.Run("should have Close wait for the messages queued before Unsubscribe", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 1 {
				close(started)
				<-release
			}
			return r.handle(ctx, topic, msg)
		}, nil))
		for msg := 1; msg <= 3; msg++ {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		waitFor(t, started, "the handler")
		testutil.NoError(t, p.Unsubscribe("s"))

		closed := make(chan error, 1)
		go func() { closed <- p.Close() }()
		eventually(t, "Close to start", func() bool {
			return errors.Is(p.Publish(context.Background(), "probe", 0), pubsub.ErrClosed)
		})
		notDone(t, closed, "Close")
		close(release)
		testutil.NoError(t, waitFor(t, closed, "Close"))
		testutil.Equal(t, "handled", r.handled, []int{1, 2, 3})
	})
}

// ---------------------------------------------------------------------------
// Publish and delivery
// ---------------------------------------------------------------------------

func Test_Publish(t *testing.T) {
	t.Run("should deliver in order to every matching subscription", func(t *testing.T) {
		p := pubsub.New[string, int]()
		a, b, c := &recorder{}, &recorder{}, &recorder{}
		testutil.NoError(t, p.Subscribe("a", "t.*", a.handle, nil))
		testutil.NoError(t, p.Subscribe("b", "t.**", b.handle, nil))
		testutil.NoError(t, p.Subscribe("c", "u", c.handle, nil))

		want := make([]int, 1000)
		for i := range want {
			want[i] = i
			testutil.NoError(t, p.Publish(context.Background(), "t.x", i))
		}
		testutil.NoError(t, p.Close())

		testutil.Equal(t, "a handled", a.handled, want)
		testutil.Equal(t, "b handled", b.handled, want)
		testutil.Equal(t, "c handled", c.handled, []int(nil))
	})

	t.Run("should fail, without queueing, on a done context", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, r.onError))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		testutil.Error(t, p.Publish(ctx, "a", 1), context.Canceled,
			"unable to publish: context canceled", map[string]any{"topic": "a"})
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int(nil))
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should skip, and report, a message whose context is done before handling", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		release := make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 1 {
				<-release
			}
			return r.handle(ctx, topic, msg)
		}, r.onError))
		ctx, cancel := context.WithCancel(context.Background())
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Publish(ctx, "a", 2))
		cancel()
		close(release)
		testutil.NoError(t, p.Close())

		testutil.Equal(t, "handled", r.handled, []int{1})
		testutil.Equal(t, "failures", r.failures, []failure{{"a", 2, context.Canceled}})
	})
}

func Test_PublishChecks(t *testing.T) {
	t.Run("should not fail on a topic no subscription matches", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "b", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int(nil))
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should fail on a context whose deadline is exceeded", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, r.onError))
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
		defer cancel()
		testutil.Error(t, p.Publish(ctx, "a", 1), context.DeadlineExceeded,
			"unable to publish: context deadline exceeded", map[string]any{"topic": "a"})
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int(nil))
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should check the topic before the context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		testutil.Error(t, (&PubSub{}).Publish(ctx, "", 1), pubsub.ErrInvalidTopic,
			"unable to publish: invalid pubsub topic", map[string]any{"topic": ""})
	})

	t.Run("should check the topic before the PubSub state", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Close())
		testutil.Error(t, p.Publish(context.Background(), "a.*", 1), pubsub.ErrInvalidTopic,
			"unable to publish: invalid pubsub topic", map[string]any{"topic": "a.*"})
	})

	t.Run("should fail with ErrClosed on a nil context once closed", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Close())
		var nilCtx context.Context
		testutil.Error(t, p.Publish(nilCtx, "a", 1), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
	})
}

func Test_Delivery(t *testing.T) {
	t.Run("should call the handler with the context given to Publish", func(t *testing.T) {
		p := &PubSub{}
		got := make(chan any, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, _ string, _ int) error {
			got <- ctx.Value(ctxKey{})
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.WithValue(context.Background(), ctxKey{}, "value"), "a", 1))
		testutil.Equal(t, "context value", waitFor(t, got, "the handler"), any("value"))
		testutil.NoError(t, p.Close())
	})

	t.Run("should replace a nil context with context.Background", func(t *testing.T) {
		p := &PubSub{}
		got := make(chan context.Context, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, _ string, _ int) error {
			got <- ctx
			return nil
		}, nil))
		var nilCtx context.Context
		testutil.NoError(t, p.Publish(nilCtx, "a", 1))
		ctx := waitFor(t, got, "the handler")
		if ctx != context.Background() {
			t.Errorf("ctx = %v, want context.Background()", ctx)
		}
		testutil.NoError(t, p.Close())
	})

	t.Run("should call the handler with the published topic, not the pattern", func(t *testing.T) {
		p := &PubSub{}
		var mu sync.Mutex
		var topics []string
		testutil.NoError(t, p.Subscribe("s", "a.*.**", func(_ context.Context, topic string, _ int) error {
			mu.Lock()
			defer mu.Unlock()
			topics = append(topics, topic)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a.b.c", 1))
		testutil.NoError(t, p.Publish(context.Background(), "a.x.y.z", 2))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "topics", topics, []string{"a.b.c", "a.x.y.z"})
	})

	t.Run("should deliver the message as is, sharing the data a pointer refers to", func(t *testing.T) {
		p := &pubsub.PubSub[string, *int]{}
		got := make(chan *int, 2)
		handler := func(_ context.Context, _ string, msg *int) error {
			got <- msg
			return nil
		}
		testutil.NoError(t, p.Subscribe("a", "t", handler, nil))
		testutil.NoError(t, p.Subscribe("b", "t", handler, nil))
		msg := new(int)
		testutil.NoError(t, p.Publish(context.Background(), "t", msg))
		testutil.NoError(t, p.Close())
		for range 2 {
			if !testutil.SamePointer(<-got, msg) {
				t.Error("the handler did not get the published pointer")
			}
		}
	})

	t.Run("should not wait for the handlers, queueing without bound", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				close(started)
				<-release
			}
			return r.handle(ctx, topic, msg)
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		waitFor(t, started, "the handler")

		const count = 10000
		published := make(chan struct{})
		go func() {
			defer close(published)
			for msg := 1; msg < count; msg++ {
				_ = p.Publish(context.Background(), "a", msg)
			}
		}()
		waitFor(t, published, "Publish to return while the handler is blocked")
		close(release)
		testutil.NoError(t, p.Close())

		want := make([]int, count)
		for i := range want {
			want[i] = i
		}
		testutil.Equal(t, "handled", r.handled, want)
	})

	t.Run("should run different subscriptions concurrently", func(t *testing.T) {
		p := &PubSub{}
		bHandled := make(chan struct{})
		aResult := make(chan bool, 1)
		testutil.NoError(t, p.Subscribe("a", "t", func(context.Context, string, int) error {
			select {
			case <-bHandled:
				aResult <- true
			case <-time.After(2 * time.Second):
				aResult <- false
			}
			return nil
		}, nil))
		testutil.NoError(t, p.Subscribe("b", "t", func(context.Context, string, int) error {
			close(bHandled)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "t", 1))
		if !waitFor(t, aResult, "the a handler") {
			t.Error("b was not handled while a was handling")
		}
		testutil.NoError(t, p.Close())
	})

	t.Run("should handle the messages of a subscription one at a time", func(t *testing.T) {
		p := &PubSub{}
		var running, overlaps atomic.Int32
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			if running.Add(1) > 1 {
				overlaps.Add(1)
			}
			runtime.Gosched()
			running.Add(-1)
			return nil
		}, nil))
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for msg := range 200 {
					_ = p.Publish(context.Background(), "a", msg)
				}
			})
		}
		wg.Wait()
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "overlaps", overlaps.Load(), int32(0))
	})

	t.Run("should report failures in publication order", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "**", func(_ context.Context, _ string, msg int) error {
			if msg%2 == 0 {
				return errMsg(msg)
			}
			return nil
		}, r.onError))
		for msg := range 6 {
			testutil.NoError(t, p.Publish(context.Background(), "t", msg))
		}
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", r.failures, []failure{
			{"t", 0, errMsg(0)}, {"t", 2, errMsg(2)}, {"t", 4, errMsg(4)},
		})
	})

	t.Run("should report to the error callback of the failing subscription only", func(t *testing.T) {
		p := &PubSub{}
		a, b := &recorder{}, &recorder{}
		cause := errors.New("boom")
		testutil.NoError(t, p.Subscribe("a", "t", func(context.Context, string, int) error { return cause }, a.onError))
		testutil.NoError(t, p.Subscribe("b", "t", b.handle, b.onError))
		testutil.NoError(t, p.Publish(context.Background(), "t", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "a failures", a.failures, []failure{{"t", 1, cause}})
		testutil.Equal(t, "b failures", b.failures, []failure(nil))
		testutil.Equal(t, "b handled", b.handled, []int{1})
	})
}

func Test_ContextDone(t *testing.T) {
	t.Run("should pass the context error as is, without calling the handler", func(t *testing.T) {
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errors.New("custom")} {
			p := &PubSub{}
			r := &recorder{}
			started, release := make(chan struct{}), make(chan struct{})
			testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
				if msg == 1 {
					close(started)
					<-release
				}
				return r.handle(ctx, topic, msg)
			}, r.onError))
			ctx := &switchCtx{Context: context.Background()}
			testutil.NoError(t, p.Publish(context.Background(), "a", 1))
			waitFor(t, started, "the handler")
			testutil.NoError(t, p.Publish(ctx, "a", 2))
			testutil.NoError(t, p.Publish(context.Background(), "a", 3))
			ctx.set(cause)
			close(release)
			testutil.NoError(t, p.Close())

			testutil.Equal(t, "handled", r.handled, []int{1, 3})
			testutil.Equal(t, "failures", len(r.failures), 1)
			if r.failures[0].err != cause {
				t.Errorf("err = %v, want %v as is", r.failures[0].err, cause)
			}
			testutil.Equal(t, "failure", r.failures[0].topic, "a")
			testutil.Equal(t, "failure msg", r.failures[0].msg, 2)
		}
	})

	t.Run("should drop the context error when the error callback is nil, and go on", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 1 {
				close(started)
				<-release
			}
			return r.handle(ctx, topic, msg)
		}, nil))
		ctx, cancel := context.WithCancel(context.Background())
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		waitFor(t, started, "the handler")
		testutil.NoError(t, p.Publish(ctx, "a", 2))
		testutil.NoError(t, p.Publish(context.Background(), "a", 3))
		cancel()
		close(release)
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{1, 3})
	})

	t.Run("should still call the handler when the context is done during handling", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		ctx, cancel := context.WithCancel(context.Background())
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, _ string, _ int) error {
			cancel()
			return ctx.Err()
		}, r.onError))
		testutil.NoError(t, p.Publish(ctx, "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", r.failures, []failure{{"a", 1, context.Canceled}})
	})
}

// errMsg is an error comparable by value, telling messages apart.
type errMsg int

func (e errMsg) Error() string { return "failed" }

func Test_HandlerFailures(t *testing.T) {
	t.Run("should report the handler error as is", func(t *testing.T) {
		cause := errors.New("boom")
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error { return cause }, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", r.failures, []failure{{"a", 1, cause}})
	})

	t.Run("should not report a nil pointer error", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			var err *typedNilError
			return err
		}, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should recover and report a panic, and go on", func(t *testing.T) {
		cause := errors.New("boom")
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			switch msg {
			case 1:
				panic("text")
			case 2:
				panic(cause)
			}
			return r.handle(ctx, topic, msg)
		}, r.onError))
		for msg := range 4 {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		testutil.NoError(t, p.Close())

		testutil.Equal(t, "handled", r.handled, []int{0, 3})
		testutil.Equal(t, "failures", len(r.failures), 2)
		assertPanic(t, r.failures[0].err,
			"unable to handle message: pubsub handler panicked", map[string]any{"id": "s", "topic": "a", "panic": "text"})
		assertPanic(t, r.failures[1].err,
			"unable to handle message: pubsub handler panicked: boom", map[string]any{"id": "s", "topic": "a", "panic": cause})
		if !errors.Is(r.failures[1].err, cause) {
			t.Error("the panic error is not wrapped")
		}
	})

	t.Run("should report a Goexit, and go on", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 1 {
				runtime.Goexit()
			}
			return r.handle(ctx, topic, msg)
		}, r.onError))
		for msg := range 3 {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		testutil.NoError(t, p.Close())

		testutil.Equal(t, "handled", r.handled, []int{0, 2})
		testutil.Equal(t, "failures", len(r.failures), 1)
		testutil.Error(t, r.failures[0].err, pubsub.ErrHandlerExit,
			"unable to handle message: pubsub handler exited", map[string]any{"id": "s", "topic": "a"})
	})

	t.Run("should survive a panic or a Goexit in the error callback", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg < 2 {
				return errors.New("boom")
			}
			return r.handle(ctx, topic, msg)
		}, func(_ string, msg int, _ error) {
			if msg == 0 {
				panic("callback")
			}
			runtime.Goexit()
		}))
		for msg := range 3 {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{2})
	})
}

func Test_HandlerFailuresMore(t *testing.T) {
	t.Run("should report a returned *flam.Error as is", func(t *testing.T) {
		cause := flam.NewWrappedError(errors.New("boom"), "handler")
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error { return cause }, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", len(r.failures), 1)
		if r.failures[0].err != error(cause) {
			t.Errorf("err = %v, want the returned error as is", r.failures[0].err)
		}
	})

	t.Run("should report a panic with a non-error value in the panic field", func(t *testing.T) {
		type value struct{ code int }
		for _, v := range []any{42, value{7}, []string{"a"}} {
			p := &PubSub{}
			r := &recorder{}
			testutil.NoError(t, p.Subscribe("s", "a.*", func(context.Context, string, int) error { panic(v) }, r.onError))
			testutil.NoError(t, p.Publish(context.Background(), "a.b", 1))
			testutil.NoError(t, p.Close())
			testutil.Equal(t, "failures", len(r.failures), 1)
			testutil.Equal(t, "topic", r.failures[0].topic, "a.b")
			testutil.Equal(t, "msg", r.failures[0].msg, 1)
			assertPanic(t, r.failures[0].err,
				"unable to handle message: pubsub handler panicked", map[string]any{"id": "s", "topic": "a.b", "panic": v})
		}
	})

	t.Run("should report a panic with a nil value as the runtime error", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error { panic(nil) }, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "failures", len(r.failures), 1)
		var nilErr *runtime.PanicNilError
		if !errors.As(r.failures[0].err, &nilErr) {
			t.Fatalf("err = %v, want it to wrap a *runtime.PanicNilError", r.failures[0].err)
		}
		assertPanic(t, r.failures[0].err,
			"unable to handle message: pubsub handler panicked: "+nilErr.Error(),
			map[string]any{"id": "s", "topic": "a", "panic": nilErr})
	})

	t.Run("should report the id of the subscription whose handler failed", func(t *testing.T) {
		p := &pubsub.PubSub[int, int]{}
		var mu sync.Mutex
		var errs []error
		testutil.NoError(t, p.Subscribe(7, "a", func(_ context.Context, _ string, msg int) error {
			if msg == 1 {
				panic("text")
			}
			runtime.Goexit()
			return nil
		}, func(_ string, _ int, err error) {
			mu.Lock()
			defer mu.Unlock()
			errs = append(errs, err)
		}))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Publish(context.Background(), "a", 2))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "errors", len(errs), 2)
		assertPanic(t, errs[0],
			"unable to handle message: pubsub handler panicked", map[string]any{"id": 7, "topic": "a", "panic": "text"})
		testutil.Error(t, errs[1], pubsub.ErrHandlerExit,
			"unable to handle message: pubsub handler exited", map[string]any{"id": 7, "topic": "a"})
	})

	t.Run("should survive consecutive Goexit calls, in order", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg%2 == 1 {
				runtime.Goexit()
			}
			return r.handle(ctx, topic, msg)
		}, r.onError))
		for msg := range 7 {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{0, 2, 4, 6})
		testutil.Equal(t, "failures", len(r.failures), 3)
		for i, f := range r.failures {
			testutil.Equal(t, "failure msg", f.msg, 2*i+1)
			testutil.Error(t, f.err, pubsub.ErrHandlerExit,
				"unable to handle message: pubsub handler exited", map[string]any{"id": "s", "topic": "a"})
		}
	})

	t.Run("should drop failures when the error callback is nil, and go on", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			switch msg {
			case 0:
				return errors.New("boom")
			case 1:
				panic("text")
			case 2:
				runtime.Goexit()
			}
			return r.handle(ctx, topic, msg)
		}, nil))
		for msg := range 4 {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{3})
	})

	t.Run("should survive an error callback panicking while reporting a handler panic", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				panic("handler")
			}
			return r.handle(ctx, topic, msg)
		}, func(string, int, error) { panic("callback") }))
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{1})
	})

	t.Run("should survive an error callback panicking or exiting while reporting a handler Goexit", func(t *testing.T) {
		for _, callback := range []func(string, int, error){
			func(string, int, error) { panic("callback") },
			func(string, int, error) { runtime.Goexit() },
		} {
			p := &PubSub{}
			r := &recorder{}
			testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
				if msg == 0 {
					runtime.Goexit()
				}
				return r.handle(ctx, topic, msg)
			}, callback))
			testutil.NoError(t, p.Publish(context.Background(), "a", 0))
			testutil.NoError(t, p.Publish(context.Background(), "a", 1))
			testutil.NoError(t, p.Close())
			testutil.Equal(t, "handled", r.handled, []int{1})
		}
	})

	t.Run("should survive an error callback exiting while reporting a done context", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				close(started)
				<-release
			}
			return r.handle(ctx, topic, msg)
		}, func(string, int, error) { runtime.Goexit() }))
		ctx, cancel := context.WithCancel(context.Background())
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		waitFor(t, started, "the handler")
		testutil.NoError(t, p.Publish(ctx, "a", 1))
		testutil.NoError(t, p.Publish(context.Background(), "a", 2))
		cancel()
		close(release)
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{0, 2})
	})
}

// ---------------------------------------------------------------------------
// handlers using their PubSub
// ---------------------------------------------------------------------------

func Test_Reentrancy(t *testing.T) {
	t.Run("should let a handler publish on its own topic, delivering after it returns", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		done := make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			_ = r.handle(ctx, topic, msg)
			if msg == 3 {
				close(done)
				return nil
			}
			return p.Publish(ctx, "a", msg+1)
		}, r.onError))
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		waitFor(t, done, "the last message")
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{0, 1, 2, 3})
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should let a handler subscribe a new subscription", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		subscribed := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			subscribed <- p.Subscribe("t", "b", r.handle, nil)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, waitFor(t, subscribed, "the handler"))
		testutil.NoError(t, p.Publish(context.Background(), "b", 2))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{2})
	})

	t.Run("should refuse a handler subscribing with its own id", func(t *testing.T) {
		p := &PubSub{}
		subscribed := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			subscribed <- p.Subscribe("s", "b", func(context.Context, string, int) error { return nil }, nil)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.Error(t, waitFor(t, subscribed, "the handler"), pubsub.ErrDuplicateID,
			"unable to subscribe: pubsub subscription id already in use", map[string]any{"id": "s", "pattern": "b"})
		testutil.NoError(t, p.Close())
	})

	t.Run("should let a handler unsubscribe another subscription", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		unsubscribed := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("other", "b", r.handle, nil))
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			unsubscribed <- p.Unsubscribe("other")
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, waitFor(t, unsubscribed, "the handler"))
		testutil.NoError(t, p.Publish(context.Background(), "b", 2))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int(nil))
	})

	t.Run("should handle the queued messages of a handler unsubscribing itself", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		release, unsubscribed := make(chan struct{}), make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				<-release
				unsubscribed <- p.Unsubscribe("s")
			}
			return r.handle(ctx, topic, msg)
		}, r.onError))
		for msg := range 3 {
			testutil.NoError(t, p.Publish(context.Background(), "a", msg))
		}
		close(release)
		testutil.NoError(t, waitFor(t, unsubscribed, "the handler"))
		testutil.NoError(t, p.Publish(context.Background(), "a", 3))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{0, 1, 2})
		testutil.Equal(t, "failures", r.failures, []failure(nil))
	})

	t.Run("should let an error callback use its PubSub", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		done := make(chan error, 3)
		testutil.NoError(t, p.Subscribe("other", "b", r.handle, nil))
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			return errors.New("boom")
		}, func(string, int, error) {
			done <- p.Publish(context.Background(), "b", 1)
			done <- p.Subscribe("new", "c", r.handle, nil)
			done <- p.Unsubscribe("new")
		}))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		for range 3 {
			testutil.NoError(t, waitFor(t, done, "the error callback"))
		}
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{1})
	})
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func Test_Close(t *testing.T) {
	t.Run("should close the PubSub", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		testutil.NoError(t, p.Subscribe("s", "a", r.handle, nil))
		testutil.NoError(t, p.Close())
		testutil.NoError(t, p.Close())

		testutil.Error(t, p.Subscribe("t", "a", r.handle, nil), pubsub.ErrClosed,
			"unable to subscribe: pubsub closed", map[string]any{"id": "t", "pattern": "a"})
		testutil.Error(t, p.Unsubscribe("s"), pubsub.ErrClosed,
			"unable to unsubscribe: pubsub closed", map[string]any{"id": "s"})
		testutil.Error(t, p.Publish(context.Background(), "a", 1), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
	})

	t.Run("should refuse to close from a handler or an error callback", func(t *testing.T) {
		p := &PubSub{}
		var mu sync.Mutex
		var errs []error
		closeFrom := func() {
			mu.Lock()
			defer mu.Unlock()
			errs = append(errs, p.Close())
		}
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			closeFrom()
			return errors.New("boom")
		}, func(string, int, error) { closeFrom() }))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Close())

		testutil.Equal(t, "errors", len(errs), 2)
		for _, err := range errs {
			testutil.Error(t, err, pubsub.ErrCloseFromHandler,
				"unable to close: pubsub closed from one of its handlers", map[string]any{})
		}
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				id := string(rune('a' + i))
				_ = p.Subscribe(id, "t.**", r.handle, nil)
				for msg := range 100 {
					_ = p.Publish(context.Background(), "t.x", msg)
				}
				_ = p.Unsubscribe(id)
			})
		}
		wg.Go(func() { _ = p.Close() })
		wg.Wait()
		testutil.NoError(t, p.Close())
	})
}

func Test_CloseMore(t *testing.T) {
	t.Run("should wait until every queued message is handled, in order", func(t *testing.T) {
		p := &PubSub{}
		a, b := &recorder{}, &recorder{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("a", "t", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				close(started)
				<-release
			}
			return a.handle(ctx, topic, msg)
		}, nil))
		testutil.NoError(t, p.Subscribe("b", "t", b.handle, nil))
		for msg := range 100 {
			testutil.NoError(t, p.Publish(context.Background(), "t", msg))
		}
		waitFor(t, started, "the handler")

		closed := make(chan error, 1)
		go func() { closed <- p.Close() }()
		eventually(t, "Close to start", func() bool {
			return errors.Is(p.Publish(context.Background(), "probe", 0), pubsub.ErrClosed)
		})
		notDone(t, closed, "Close")
		close(release)
		testutil.NoError(t, waitFor(t, closed, "Close"))

		want := make([]int, 100)
		for i := range want {
			want[i] = i
		}
		testutil.Equal(t, "a handled", a.handled, want)
		testutil.Equal(t, "b handled", b.handled, want)
	})

	t.Run("should skip, and report, the queued messages whose context is done", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				close(started)
				<-release
			}
			return r.handle(ctx, topic, msg)
		}, r.onError))
		ctx, cancel := context.WithCancel(context.Background())
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		waitFor(t, started, "the handler")
		testutil.NoError(t, p.Publish(ctx, "a", 1))
		testutil.NoError(t, p.Publish(ctx, "a", 2))
		testutil.NoError(t, p.Publish(context.Background(), "a", 3))

		closed := make(chan error, 1)
		go func() { closed <- p.Close() }()
		cancel()
		close(release)
		testutil.NoError(t, waitFor(t, closed, "Close"))
		testutil.Equal(t, "handled", r.handled, []int{0, 3})
		testutil.Equal(t, "failures", r.failures, []failure{{"a", 1, context.Canceled}, {"a", 2, context.Canceled}})
	})

	t.Run("should make the methods fail with ErrClosed while it drains, handlers included", func(t *testing.T) {
		p := &PubSub{}
		started, release := make(chan struct{}), make(chan struct{})
		errs := make(chan error, 3)
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			close(started)
			<-release
			errs <- p.Subscribe("t", "b", func(context.Context, string, int) error { return nil }, nil)
			errs <- p.Unsubscribe("s")
			errs <- p.Publish(context.Background(), "b", 2)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		waitFor(t, started, "the handler")

		closed := make(chan error, 1)
		go func() { closed <- p.Close() }()
		eventually(t, "Close to start", func() bool {
			return errors.Is(p.Publish(context.Background(), "probe", 0), pubsub.ErrClosed)
		})
		close(release)
		testutil.Error(t, waitFor(t, errs, "Subscribe"), pubsub.ErrClosed,
			"unable to subscribe: pubsub closed", map[string]any{"id": "t", "pattern": "b"})
		testutil.Error(t, waitFor(t, errs, "Unsubscribe"), pubsub.ErrClosed,
			"unable to unsubscribe: pubsub closed", map[string]any{"id": "s"})
		testutil.Error(t, waitFor(t, errs, "Publish"), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "b"})
		testutil.NoError(t, waitFor(t, closed, "Close"))
	})

	t.Run("should close nothing when refused from a handler", func(t *testing.T) {
		p := &PubSub{}
		r := &recorder{}
		refused := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(ctx context.Context, topic string, msg int) error {
			if msg == 0 {
				refused <- p.Close()
			}
			return r.handle(ctx, topic, msg)
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		testutil.Error(t, waitFor(t, refused, "the handler"), pubsub.ErrCloseFromHandler,
			"unable to close: pubsub closed from one of its handlers", map[string]any{})

		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, p.Subscribe("t", "b", r.handle, nil))
		testutil.NoError(t, p.Unsubscribe("t"))
		testutil.NoError(t, p.Close())
		testutil.Equal(t, "handled", r.handled, []int{0, 1})
	})

	t.Run("should refuse to close from a handler of another subscription", func(t *testing.T) {
		p := &PubSub{}
		refused := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("other", "b", func(context.Context, string, int) error { return nil }, nil))
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			refused <- p.Close()
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.Error(t, waitFor(t, refused, "the handler"), pubsub.ErrCloseFromHandler,
			"unable to close: pubsub closed from one of its handlers", map[string]any{})
		testutil.NoError(t, p.Close())
	})

	t.Run("should refuse to close from an error callback reporting a done context", func(t *testing.T) {
		p := &PubSub{}
		started, release := make(chan struct{}), make(chan struct{})
		refused := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(_ context.Context, _ string, msg int) error {
			if msg == 0 {
				close(started)
				<-release
			}
			return nil
		}, func(string, int, error) { refused <- p.Close() }))
		ctx, cancel := context.WithCancel(context.Background())
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		waitFor(t, started, "the handler")
		testutil.NoError(t, p.Publish(ctx, "a", 1))
		cancel()
		close(release)
		testutil.Error(t, waitFor(t, refused, "the error callback"), pubsub.ErrCloseFromHandler,
			"unable to close: pubsub closed from one of its handlers", map[string]any{})
		testutil.NoError(t, p.Close())
	})

	t.Run("should refuse to close from a handler running after a Goexit", func(t *testing.T) {
		p := &PubSub{}
		refused := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(_ context.Context, _ string, msg int) error {
			if msg == 0 {
				runtime.Goexit()
			}
			refused <- p.Close()
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 0))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.Error(t, waitFor(t, refused, "the handler"), pubsub.ErrCloseFromHandler,
			"unable to close: pubsub closed from one of its handlers", map[string]any{})
		testutil.NoError(t, p.Close())
	})

	t.Run("should not refuse a Close of another PubSub from a handler", func(t *testing.T) {
		p, other := &PubSub{}, &PubSub{}
		closed := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			closed <- other.Close()
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, waitFor(t, closed, "the handler"))
		testutil.Error(t, other.Publish(context.Background(), "a", 1), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
		testutil.NoError(t, p.Close())
	})

	t.Run("should only detect the callback goroutine, not those it starts", func(t *testing.T) {
		p := &PubSub{}
		closed := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, int) error {
			go func() { closed <- p.Close() }()
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", 1))
		testutil.NoError(t, waitFor(t, closed, "the Close of the started goroutine"))
		testutil.Error(t, p.Publish(context.Background(), "a", 1), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
	})

	t.Run("should return nil on every later call", func(t *testing.T) {
		p := &PubSub{}
		testutil.NoError(t, p.Subscribe("s", "a", (&recorder{}).handle, nil))
		for range 3 {
			testutil.NoError(t, p.Close())
		}
	})
}

func Test_PublishCheckOrder(t *testing.T) {
	t.Run("should check the context before the closed state", func(t *testing.T) {
		p := pubsub.New[string, string]()
		testutil.NoError(t, p.Close())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := p.Publish(ctx, "a", "x")
		testutil.Error(t, err, context.Canceled, "unable to publish: context canceled",
			map[string]any{"topic": "a"})
	})
}

// assertPanic fails the test unless err is a *flam.Error wrapping
// pubsub.ErrHandlerPanic, with the given text and context, plus the stack
// of the panicking handler.
func assertPanic(t *testing.T, err error, text string, ctx map[string]any) {
	t.Helper()
	if !errors.Is(err, pubsub.ErrHandlerPanic) {
		t.Fatalf("error = %v, want it to wrap ErrHandlerPanic", err)
	}
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T, want a *flam.Error", err)
	}
	testutil.Equal(t, "Error()", err.Error(), text)
	got := e.Context()
	stack, _ := got[pubsub.ErrorFieldStack].(string)
	if !strings.Contains(stack, "pubsub_test.go") {
		t.Errorf("stack does not show the panicking handler:\n%s", stack)
	}
	delete(got, pubsub.ErrorFieldStack)
	testutil.Equal(t, "Context()", got, ctx)
}
