package pubsub

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/pubsub"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Bundle(t *testing.T) {
	t.Run("should provide a single IPubSub", func(t *testing.T) {
		b := pubsub.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.pubsub")

		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0"})

		var first, second flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(p flam.IPubSub[string, string]) { first = p }))
		testutil.NoError(t, app.Invoke(func(p flam.IPubSub[string, string]) { second = p }))
		if first == nil || first != second {
			t.Fatal("the container did not provide a single IPubSub")
		}

		got := make(chan string, 1)
		testutil.NoError(t, first.Subscribe("s", "a", func(_ context.Context, _ string, msg string) error {
			got <- msg
			return nil
		}, nil))
		testutil.NoError(t, first.Publish(context.Background(), "a", "hello"))
		testutil.Equal(t, "message", <-got, "hello")
		testutil.NoError(t, first.Close())
	})

	t.Run("should close the provided IPubSub", func(t *testing.T) {
		b := pubsub.NewBundle()
		closable, ok := b.(flam.IClosableBundle)
		if !ok {
			t.Fatal("the bundle does not implement IClosableBundle")
		}

		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		var p flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { p = provided }))

		testutil.NoError(t, closable.Close(app))
		testutil.Error(t, p.Publish(context.Background(), "a", "hello"), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
		testutil.NoError(t, closable.Close(app))
	})

	t.Run("should create and close the IPubSub when never used", func(t *testing.T) {
		b := pubsub.NewBundle()
		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))

		var p flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { p = provided }))
		testutil.Error(t, p.Publish(context.Background(), "a", "x"), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
	})

	t.Run("should return a new bundle on each call", func(t *testing.T) {
		first, second := pubsub.NewBundle(), pubsub.NewBundle()
		if first == second {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		testutil.Equal(t, "Id()", second.Id(), first.Id())
	})

	t.Run("should provide a *pubsub.PubSub with string ids and messages", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, pubsub.NewBundle().Register(app))
		var p flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { p = provided }))
		if _, ok := p.(*pubsub.PubSub[string, string]); !ok {
			t.Fatalf("provided = %T, want *pubsub.PubSub[string, string]", p)
		}
		testutil.NoError(t, p.Close())
	})

	t.Run("should not provide an IPubSub of other types", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, pubsub.NewBundle().Register(app))
		if err := app.Invoke(func(flam.IPubSub[string, int]) {}); err == nil {
			t.Fatal("the container provided an IPubSub[string, int]")
		}
	})

	t.Run("should fail to register when the application refuses the provider", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return cause }
		if err := pubsub.NewBundle().Register(app); !errors.Is(err, cause) {
			t.Fatalf("error = %v, want it to wrap %v", err, cause)
		}
	})

	t.Run("should provide a distinct IPubSub to each application", func(t *testing.T) {
		b1, b2 := pubsub.NewBundle(), pubsub.NewBundle()
		app1, app2 := testutil.NewApp(), testutil.NewApp()
		testutil.NoError(t, b1.Register(app1))
		testutil.NoError(t, b2.Register(app2))
		var p1, p2 flam.IPubSub[string, string]
		testutil.NoError(t, app1.Invoke(func(p flam.IPubSub[string, string]) { p1 = p }))
		testutil.NoError(t, app2.Invoke(func(p flam.IPubSub[string, string]) { p2 = p }))
		if p1 == p2 {
			t.Fatal("both applications got the same IPubSub")
		}

		testutil.NoError(t, b1.(flam.IClosableBundle).Close(app1))
		testutil.Error(t, p1.Publish(context.Background(), "a", "x"), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
		testutil.NoError(t, p2.Publish(context.Background(), "a", "x"))
		testutil.NoError(t, b2.(flam.IClosableBundle).Close(app2))
	})

	t.Run("should wait for the queued messages when closing", func(t *testing.T) {
		b := pubsub.NewBundle()
		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		var p flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { p = provided }))

		var mu sync.Mutex
		var handled []string
		started, release := make(chan struct{}), make(chan struct{})
		testutil.NoError(t, p.Subscribe("s", "a", func(_ context.Context, _ string, msg string) error {
			if msg == "first" {
				close(started)
				<-release
			}
			mu.Lock()
			defer mu.Unlock()
			handled = append(handled, msg)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", "first"))
		testutil.NoError(t, p.Publish(context.Background(), "a", "second"))
		<-started

		closed := make(chan error, 1)
		go func() { closed <- b.(flam.IClosableBundle).Close(app) }()
		deadline := time.Now().Add(2 * time.Second)
		for !errors.Is(p.Publish(context.Background(), "probe", ""), pubsub.ErrClosed) {
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for Close to start")
			}
			runtime.Gosched()
		}
		select {
		case err := <-closed:
			t.Fatalf("Close() = %v before the queued messages were handled", err)
		default:
		}
		close(release)
		select {
		case err := <-closed:
			testutil.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for Close")
		}
		testutil.Equal(t, "handled", handled, []string{"first", "second"})
	})

	t.Run("should keep providing the closed IPubSub after Close", func(t *testing.T) {
		b := pubsub.NewBundle()
		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		var before, after flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { before = provided }))
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { after = provided }))
		if before != after {
			t.Fatal("the container provided a new IPubSub after Close")
		}
		testutil.Error(t, after.Publish(context.Background(), "a", "x"), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
	})

	t.Run("should return the error of a Close refused from a handler, and close later", func(t *testing.T) {
		b := pubsub.NewBundle()
		closable := b.(flam.IClosableBundle)
		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		var p flam.IPubSub[string, string]
		testutil.NoError(t, app.Invoke(func(provided flam.IPubSub[string, string]) { p = provided }))

		refused := make(chan error, 1)
		testutil.NoError(t, p.Subscribe("s", "a", func(context.Context, string, string) error {
			refused <- closable.Close(app)
			return nil
		}, nil))
		testutil.NoError(t, p.Publish(context.Background(), "a", "x"))
		select {
		case err := <-refused:
			testutil.Error(t, err, pubsub.ErrCloseFromHandler,
				"unable to close: pubsub closed from one of its handlers", map[string]any{})
		case <-time.After(2 * time.Second):
			t.Fatal("the handler was not called")
		}

		testutil.NoError(t, closable.Close(app))
		testutil.Error(t, p.Publish(context.Background(), "a", "x"), pubsub.ErrClosed,
			"unable to publish: pubsub closed", map[string]any{"topic": "a"})
		testutil.NoError(t, closable.Close(app))
	})
}
