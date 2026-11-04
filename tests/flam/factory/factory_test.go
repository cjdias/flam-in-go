package factory

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

type Bag = container.Bag

// resource is a resource implementing io.Closer, failing its Close with
// closeErr, and appending its id to order, when set, on Close.
type resource struct {
	id       string
	closed   atomic.Int32
	closeErr error
	order    *[]string
}

func (r *resource) Id() string { return r.id }

func (r *resource) Close() error {
	r.closed.Add(1)
	if r.order != nil {
		*r.order = append(*r.order, r.id)
	}
	return r.closeErr
}

// plain is a resource not implementing io.Closer.
type plain struct {
	id string
}

func (p *plain) Id() string { return p.id }

// creator accepts the configurations whose "driver" field is driver, and
// creates resources with create, or a *resource for the configured id.
type creator struct {
	driver   string
	create   func(bag flam.IBag) (*resource, error)
	accepted atomic.Int32
	created  atomic.Int32
}

func (c *creator) Accept(bag flam.IBag) bool {
	c.accepted.Add(1)
	return bag.String("driver") == c.driver
}

func (c *creator) Create(bag flam.IBag) (*resource, error) {
	c.created.Add(1)
	if c.create != nil {
		return c.create(bag)
	}
	return &resource{id: bag.String(factory.ConfigFieldId)}, nil
}

// newFactory returns a factory over config with the given creators,
// failing the test on error.
func newFactory(t *testing.T, config flam.IBag, creators ...flam.IResourceCreator[*resource]) flam.IFactory[*resource] {
	t.Helper()
	f, err := factory.New(config, creators...)
	testutil.NoError(t, err)
	return f
}

// funcCreator is a creator of any resource type, delegating Accept, when
// set, else accepting every configuration, and Create to functions.
type funcCreator[R flam.IResource] struct {
	accept func(bag flam.IBag) bool
	create func(bag flam.IBag) (R, error)
}

func (c *funcCreator[R]) Accept(bag flam.IBag) bool {
	if c.accept == nil {
		return true
	}
	return c.accept(bag)
}

func (c *funcCreator[R]) Create(bag flam.IBag) (R, error) {
	return c.create(bag)
}

// rawBag is a fake flam.IBag whose Get("") returns root; its other methods
// are not implemented.
type rawBag struct {
	flam.IBag

	root any
}

func (b *rawBag) Get(path string, def ...any) any {
	if path == "" {
		return b.root
	}
	panic("rawBag.Get called with a path: " + path)
}

// newAnyFactory returns a factory of flam.IResource values over config with
// the given creators, failing the test on error.
func newAnyFactory(t *testing.T, config flam.IBag, creators ...flam.IResourceCreator[flam.IResource]) flam.IFactory[flam.IResource] {
	t.Helper()
	f, err := factory.New(config, creators...)
	testutil.NoError(t, err)
	return f
}

// joinedErrors returns the errors joined in err, failing the test unless
// err is an errors.Join result rather than a *flam.Error itself.
func joinedErrors(t *testing.T, err error) []error {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want joined errors")
	}
	if _, isError := err.(*flam.Error); isError {
		t.Fatalf("error = %T, want an errors.Join result", err)
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("error = %T, want an errors.Join result", err)
	}
	return joined.Unwrap()
}

// waitForWaitingGets blocks until n goroutines are waiting, in a Get call,
// for a resource another Get call is creating, as their stacks tell. It
// fails the test after a generous timeout.
func waitForWaitingGets(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for countWaitingGets() < n {
		if time.Now().After(deadline) {
			t.Fatalf("%d Get calls waiting, want %d", countWaitingGets(), n)
		}
		runtime.Gosched()
	}
}

// countWaitingGets returns the number of goroutines blocked on a channel
// receive directly in the Get method of the factory.
func countWaitingGets() int {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}

	count := 0
	for _, g := range strings.Split(string(buf), "\n\n") {
		lines := strings.SplitN(g, "\n", 3)
		if len(lines) >= 2 &&
			strings.Contains(lines[0], "[chan receive") &&
			strings.Contains(lines[1], "pkg/flam/factory.(*factoryImpl[") &&
			strings.Contains(lines[1], "]).Get(") {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ConfigFieldId", factory.ConfigFieldId, "id")
	testutil.Equal(t, "ErrorFieldID", factory.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldResourceID", factory.ErrorFieldResourceID, "resource_id")

	sentinels := []error{
		factory.ErrNilCreator, factory.ErrClosed, factory.ErrUnknownID,
		factory.ErrDuplicateID, factory.ErrNoCreator, factory.ErrNilResource,
		factory.ErrIDMismatch, factory.ErrCreatorPanic,
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
// New
// ---------------------------------------------------------------------------

func Test_New(t *testing.T) {
	t.Run("should fail on a nil creator", func(t *testing.T) {
		var nilCreator *creator
		for _, creators := range [][]flam.IResourceCreator[*resource]{
			{nil}, {&creator{}, nil}, {nilCreator},
		} {
			f, err := factory.New(Bag{}, creators...)
			testutil.Error(t, err, factory.ErrNilCreator,
				"unable to create factory: nil factory creator", map[string]any{})
			if f != nil {
				t.Error("New() returned a factory with an error")
			}
		}
	})

	t.Run("should accept a nil config and no creators", func(t *testing.T) {
		f := newFactory(t, nil)
		testutil.Equal(t, "Available()", f.Available(), []string{})
	})

	t.Run("should only keep the bag entries of the config", func(t *testing.T) {
		config := Bag{"b": Bag{}, "a": map[string]any{}, "n": Bag(nil), "x": 1, "s": "bag"}
		f := newFactory(t, config)
		testutil.Equal(t, "Available()", f.Available(), []string{"a", "b", "n"})
	})

	t.Run("should copy the config", func(t *testing.T) {
		config := Bag{"a": Bag{"driver": "x"}}
		c := &creator{driver: "x"}
		f := newFactory(t, config, c)
		config["b"] = Bag{"driver": "x"}
		config["a"].(Bag)["driver"] = "y"

		testutil.Equal(t, "Available()", f.Available(), []string{"a"})
		_, err := f.Generate("a")
		testutil.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// Generate
// ---------------------------------------------------------------------------

func Test_Generate(t *testing.T) {
	t.Run("should create a new resource with the first accepting creator", func(t *testing.T) {
		first, second, third := &creator{driver: "y"}, &creator{driver: "x"}, &creator{driver: "x"}
		f := newFactory(t, Bag{"a": Bag{"driver": "x"}}, first, second, third)

		r1, err := f.Generate("a")
		testutil.NoError(t, err)
		r2, err := f.Generate("a")
		testutil.NoError(t, err)
		if r1 == r2 {
			t.Error("Generate() returned the same resource twice")
		}
		testutil.Equal(t, "id", r1.Id(), "a")
		testutil.Equal(t, "created", [3]int32{first.created.Load(), second.created.Load(), third.created.Load()}, [3]int32{0, 2, 0})
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should give each creator method its own copy, holding the id", func(t *testing.T) {
		config := Bag{"a": Bag{"driver": "x", "id": "other", "n": Bag{"k": 1}}}
		var bags []flam.IBag
		c := &creator{driver: "x", create: func(bag flam.IBag) (*resource, error) {
			bags = append(bags, bag)
			testutil.NoError(t, bag.Set("n.k", 2))
			return &resource{id: bag.String("id")}, nil
		}}
		f := newFactory(t, config, c)

		for range 2 {
			_, err := f.Generate("a")
			testutil.NoError(t, err)
		}
		testutil.Equal(t, "creator bag", bags[1], flam.IBag(Bag{"driver": "x", "id": "a", "n": Bag{"k": 2}}))
		testutil.Equal(t, "config", config, Bag{"a": Bag{"driver": "x", "id": "other", "n": Bag{"k": 1}}})
	})

	t.Run("should give a nil bag configuration as an empty bag", func(t *testing.T) {
		c := &creator{create: func(bag flam.IBag) (*resource, error) {
			return &resource{id: bag.String("id")}, nil
		}}
		r, err := newFactory(t, Bag{"a": nil, "n": Bag(nil)}, c).Generate("n")
		testutil.NoError(t, err)
		testutil.Equal(t, "id", r.Id(), "n")
	})

	t.Run("should fail on an unknown id", func(t *testing.T) {
		_, err := newFactory(t, Bag{}).Generate("a")
		testutil.Error(t, err, factory.ErrUnknownID,
			"unable to generate resource: unknown factory resource id", map[string]any{"id": "a"})
	})

	t.Run("should fail when no creator accepts the configuration", func(t *testing.T) {
		_, err := newFactory(t, Bag{"a": Bag{}}, &creator{driver: "x"}).Generate("a")
		testutil.Error(t, err, factory.ErrNoCreator,
			"unable to generate resource: no factory creator accepts the resource", map[string]any{"id": "a"})
	})

	t.Run("should wrap the creator error", func(t *testing.T) {
		cause := errors.New("boom")
		c := &creator{create: func(flam.IBag) (*resource, error) { return nil, cause }}
		_, err := newFactory(t, Bag{"a": Bag{}}, c).Generate("a")
		testutil.Error(t, err, cause, "unable to generate resource: boom", map[string]any{"id": "a"})
	})

	t.Run("should fail on a nil resource", func(t *testing.T) {
		c := &creator{create: func(flam.IBag) (*resource, error) { return nil, nil }}
		_, err := newFactory(t, Bag{"a": Bag{}}, c).Generate("a")
		testutil.Error(t, err, factory.ErrNilResource,
			"unable to generate resource: nil factory resource", map[string]any{"id": "a"})
	})

	t.Run("should close and fail on a resource id mismatch", func(t *testing.T) {
		r := &resource{id: "b"}
		c := &creator{create: func(flam.IBag) (*resource, error) { return r, nil }}
		_, err := newFactory(t, Bag{"a": Bag{}}, c).Generate("a")
		testutil.Error(t, err, factory.ErrIDMismatch,
			"unable to generate resource: factory resource id mismatch", map[string]any{"id": "a", "resource_id": "b"})
		testutil.Equal(t, "closed", r.closed.Load(), int32(1))
	})
}

// ---------------------------------------------------------------------------
// Get, Has, Stored
// ---------------------------------------------------------------------------

func Test_Get(t *testing.T) {
	t.Run("should create the resource once and store it", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)

		r1, err := f.Get("a")
		testutil.NoError(t, err)
		r2, err := f.Get("a")
		testutil.NoError(t, err)
		if r1 != r2 {
			t.Error("Get() returned two resources")
		}
		testutil.Equal(t, "created", c.created.Load(), int32(1))
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})
		testutil.Equal(t, "Has(a)", f.Has("a"), true)
		testutil.Equal(t, "Has(b)", f.Has("b"), true)
		testutil.Equal(t, "Has(c)", f.Has("c"), false)
	})

	t.Run("should create the resource once for concurrent calls", func(t *testing.T) {
		release := make(chan struct{})
		c := &creator{create: func(bag flam.IBag) (*resource, error) {
			<-release
			return &resource{id: "a"}, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		results := make([]*resource, 10)
		var wg sync.WaitGroup
		for i := range results {
			wg.Go(func() {
				r, err := f.Get("a")
				if err != nil {
					t.Error(err)
				}
				results[i] = r
			})
		}
		waitForWaitingGets(t, len(results)-1) // all but the creating one
		close(release)
		wg.Wait()

		testutil.Equal(t, "created", c.created.Load(), int32(1))
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})
		for _, r := range results {
			if r != results[0] {
				t.Fatal("concurrent Get() calls returned distinct resources")
			}
		}
	})

	t.Run("should not store a failed creation", func(t *testing.T) {
		fail := true
		c := &creator{create: func(flam.IBag) (*resource, error) {
			if fail {
				return nil, errors.New("boom")
			}
			return &resource{id: "a"}, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		_, err := f.Get("a")
		if err == nil {
			t.Fatal("Get() = nil error, want the creator one")
		}
		testutil.Equal(t, "Stored()", f.Stored(), []string{})

		fail = false
		_, err = f.Get("a")
		testutil.NoError(t, err)
	})

	t.Run("should repanic a creator panic and fail the waiting calls", func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		c := &creator{create: func(flam.IBag) (*resource, error) {
			close(started)
			<-release
			panic("boom")
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		panicked := make(chan bool)
		go func() {
			defer func() { panicked <- recover() != nil }()
			_, _ = f.Get("a")
		}()
		<-started

		waited := make(chan error)
		go func() {
			_, err := f.Get("a")
			waited <- err
		}()
		waitForWaitingGets(t, 1)
		close(release)

		if !<-panicked {
			t.Error("the Get() call running the creator did not panic")
		}
		testutil.Error(t, <-waited, factory.ErrCreatorPanic,
			"unable to get resource: factory creator panicked", map[string]any{"id": "a"})
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should fail on an unknown id", func(t *testing.T) {
		_, err := newFactory(t, Bag{}).Get("a")
		testutil.Error(t, err, factory.ErrUnknownID,
			"unable to generate resource: unknown factory resource id", map[string]any{"id": "a"})
	})
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

func Test_Store(t *testing.T) {
	t.Run("should store a resource under any free id", func(t *testing.T) {
		f := newFactory(t, Bag{})
		r := &resource{id: "a"}
		testutil.NoError(t, f.Store("a", r))

		got, err := f.Get("a")
		testutil.NoError(t, err)
		if got != r {
			t.Error("Get() did not return the stored resource")
		}
		testutil.Equal(t, "Has(a)", f.Has("a"), true)
		testutil.Equal(t, "Available()", f.Available(), []string{})
	})

	t.Run("should fail on a nil resource", func(t *testing.T) {
		testutil.Error(t, newFactory(t, Bag{}).Store("a", nil), factory.ErrNilResource,
			"unable to store resource: nil factory resource", map[string]any{"id": "a"})
	})

	t.Run("should fail, without closing it, on an id mismatch", func(t *testing.T) {
		r := &resource{id: "b"}
		testutil.Error(t, newFactory(t, Bag{}).Store("a", r), factory.ErrIDMismatch,
			"unable to store resource: factory resource id mismatch", map[string]any{"id": "a", "resource_id": "b"})
		testutil.Equal(t, "closed", r.closed.Load(), int32(0))
	})

	t.Run("should fail on an id in use", func(t *testing.T) {
		f := newFactory(t, Bag{})
		testutil.NoError(t, f.Store("a", &resource{id: "a"}))
		testutil.Error(t, f.Store("a", &resource{id: "a"}), factory.ErrDuplicateID,
			"unable to store resource: factory resource id already in use", map[string]any{"id": "a"})
	})
}

// ---------------------------------------------------------------------------
// GenerateAll, Remove, RemoveAll
// ---------------------------------------------------------------------------

func Test_GenerateAll(t *testing.T) {
	t.Run("should get every available resource", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)
		_, err := f.Get("a")
		testutil.NoError(t, err)

		testutil.NoError(t, f.GenerateAll())
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a", "b"})
		testutil.Equal(t, "created", c.created.Load(), int32(2))
	})

	t.Run("should stop at the first failure, in id order", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{"driver": "x"}, "b": Bag{}, "c": Bag{"driver": "x"}}, &creator{driver: "x"})
		testutil.Error(t, f.GenerateAll(), factory.ErrNoCreator,
			"unable to generate resource: no factory creator accepts the resource", map[string]any{"id": "b"})
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})
	})

	t.Run("should return the error of a creator using a closed factory", func(t *testing.T) {
		dependency := newFactory(t, Bag{"d": Bag{}}, &creator{})
		testutil.NoError(t, dependency.Close())
		f := newFactory(t, Bag{"a": Bag{}}, &funcCreator[*resource]{create: func(flam.IBag) (*resource, error) {
			_, err := dependency.Get("d")
			return nil, err
		}})

		err := f.GenerateAll()
		if !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("GenerateAll() = %v, want the creator error", err)
		}
		var e *flam.Error
		errors.As(err, &e)
		testutil.Equal(t, "id", e.Context()["id"], "a")
	})
}

func Test_Remove(t *testing.T) {
	t.Run("should remove and close the resource, keeping its configuration", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}}, c)
		r1, err := f.Get("a")
		testutil.NoError(t, err)

		testutil.NoError(t, f.Remove("a"))
		testutil.Equal(t, "closed", r1.closed.Load(), int32(1))
		testutil.Equal(t, "Stored()", f.Stored(), []string{})

		r2, err := f.Get("a")
		testutil.NoError(t, err)
		if r1 == r2 {
			t.Error("Get() returned the removed resource")
		}
	})

	t.Run("should remove the resource even when its Close fails", func(t *testing.T) {
		cause := errors.New("boom")
		f := newFactory(t, Bag{})
		testutil.NoError(t, f.Store("a", &resource{id: "a", closeErr: cause}))
		testutil.Error(t, f.Remove("a"), cause, "unable to remove resource: boom", map[string]any{"id": "a"})
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should fail on an id not stored", func(t *testing.T) {
		testutil.Error(t, newFactory(t, Bag{"a": Bag{}}).Remove("a"), factory.ErrUnknownID,
			"unable to remove resource: unknown factory resource id", map[string]any{"id": "a"})
	})
}

func Test_RemoveAll(t *testing.T) {
	cause := errors.New("boom")
	f := newFactory(t, Bag{})
	a, b, c := &resource{id: "a", closeErr: cause}, &resource{id: "b"}, &resource{id: "c", closeErr: cause}
	for _, r := range []*resource{a, b, c} {
		testutil.NoError(t, f.Store(r.id, r))
	}

	err := f.RemoveAll()
	testutil.Equal(t, "error", err.Error(), "unable to remove resource: boom\nunable to remove resource: boom")
	if !errors.Is(err, cause) {
		t.Error("RemoveAll() error does not wrap the Close error")
	}
	testutil.Equal(t, "closed", [3]int32{a.closed.Load(), b.closed.Load(), c.closed.Load()}, [3]int32{1, 1, 1})
	testutil.Equal(t, "Stored()", f.Stored(), []string{})
	testutil.NoError(t, f.RemoveAll())
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func Test_Close(t *testing.T) {
	t.Run("should close the stored resources and the factory", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{}}, &creator{})
		r, err := f.Get("a")
		testutil.NoError(t, err)

		testutil.NoError(t, f.Close())
		testutil.NoError(t, f.Close())
		testutil.Equal(t, "closed", r.closed.Load(), int32(1))

		testutil.Equal(t, "Available()", f.Available(), []string(nil))
		testutil.Equal(t, "Stored()", f.Stored(), []string(nil))
		testutil.Equal(t, "Has(a)", f.Has("a"), false)

		closed := map[string]any{"id": "a"}
		_, err = f.Get("a")
		testutil.Error(t, err, factory.ErrClosed, "unable to get resource: factory closed", closed)
		_, err = f.Generate("a")
		testutil.Error(t, err, factory.ErrClosed, "unable to generate resource: factory closed", closed)
		testutil.Error(t, f.Store("a", &resource{id: "a"}), factory.ErrClosed,
			"unable to store resource: factory closed", closed)
		testutil.Error(t, f.Remove("a"), factory.ErrClosed, "unable to remove resource: factory closed", closed)
		testutil.Error(t, f.GenerateAll(), factory.ErrClosed,
			"unable to generate resources: factory closed", map[string]any{})
		testutil.Error(t, f.RemoveAll(), factory.ErrClosed,
			"unable to remove resources: factory closed", map[string]any{})
	})

	t.Run("should close a resource created by a Get running across Close", func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		r := &resource{id: "a"}
		c := &creator{create: func(flam.IBag) (*resource, error) {
			close(started)
			<-release
			return r, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		done := make(chan error)
		go func() {
			_, err := f.Get("a")
			done <- err
		}()
		<-started
		testutil.NoError(t, f.Close())
		close(release)

		testutil.Error(t, <-done, factory.ErrClosed,
			"unable to get resource: factory closed", map[string]any{"id": "a"})
		testutil.Equal(t, "closed", r.closed.Load(), int32(1))
	})
}

// ---------------------------------------------------------------------------
// New: configuration and creators
// ---------------------------------------------------------------------------

func Test_New_Config(t *testing.T) {
	scenarios := []struct {
		name   string
		config flam.IBag
		want   []string
	}{
		{"should hold no configuration from a nil IBag", nil, []string{}},
		{"should hold no configuration from a nil Bag", Bag(nil), []string{}},
		{"should hold no configuration from a nil IBag pointer", (*rawBag)(nil), []string{}},
		{"should hold no configuration from an empty Bag", Bag{}, []string{}},
		{"should read a config whose Get(\"\") returns a Bag", &rawBag{root: Bag{"a": Bag{}}}, []string{"a"}},
		{"should read a config whose Get(\"\") returns a map[string]any", &rawBag{root: map[string]any{"a": Bag{}}}, []string{"a"}},
		{"should hold no configuration when Get(\"\") returns nil", &rawBag{root: nil}, []string{}},
		{"should hold no configuration when Get(\"\") returns a scalar", &rawBag{root: "bag"}, []string{}},
		{"should hold no configuration when Get(\"\") returns another map type", &rawBag{root: map[string]string{"a": "b"}}, []string{}},
		{
			"should keep Bag, map[string]any and empty bag entries only",
			Bag{
				"bag": Bag{}, "map": map[string]any{}, "nilBag": Bag(nil), "nilMap": map[string]any(nil),
				"nil": nil, "int": 1, "string": "bag", "slice": []any{Bag{}}, "otherMap": map[string]int{},
			},
			[]string{"bag", "map", "nilBag", "nilMap"},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			f := newFactory(t, s.config)
			testutil.Equal(t, "Available()", f.Available(), s.want)
			testutil.Equal(t, "Stored()", f.Stored(), []string{})
		})
	}

	t.Run("should give an empty slot as an empty configuration holding the id", func(t *testing.T) {
		for _, id := range []string{"nilBag", "nilMap"} {
			var got flam.IBag
			c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
				got = bag
				return &resource{id: id}, nil
			}}
			f := newFactory(t, Bag{"nilBag": Bag(nil), "nilMap": map[string]any(nil)}, c)
			_, err := f.Generate(id)
			testutil.NoError(t, err)
			testutil.Equal(t, "creator bag", got, flam.IBag(Bag{factory.ConfigFieldId: id}))
		}
	})

	t.Run("should deep copy the config", func(t *testing.T) {
		config := Bag{"a": Bag{"driver": "x", "n": Bag{"k": 1}}, "m": map[string]any{"driver": "x"}}
		var got []flam.IBag
		c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
			got = append(got, bag)
			return &resource{id: bag.String(factory.ConfigFieldId)}, nil
		}}
		f := newFactory(t, config, c)
		config["a"].(Bag)["n"].(Bag)["k"] = 2
		config["m"].(map[string]any)["driver"] = "y"
		delete(config, "a")

		testutil.Equal(t, "Available()", f.Available(), []string{"a", "m"})
		for _, id := range []string{"a", "m"} {
			_, err := f.Generate(id)
			testutil.NoError(t, err)
		}
		testutil.Equal(t, "creator bags", got, []flam.IBag{
			Bag{"driver": "x", "id": "a", "n": Bag{"k": 1}},
			Bag{"driver": "x", "id": "m"},
		})
	})

	t.Run("should copy the creators", func(t *testing.T) {
		first, second := &creator{driver: "x"}, &creator{driver: "x"}
		creators := []flam.IResourceCreator[*resource]{first}
		f := newFactory(t, Bag{"a": Bag{"driver": "x"}}, creators...)
		creators[0] = second

		_, err := f.Generate("a")
		testutil.NoError(t, err)
		testutil.Equal(t, "created", [2]int32{first.created.Load(), second.created.Load()}, [2]int32{1, 0})
	})

	t.Run("should only hold stored resources without creators", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{}})
		_, err := f.Get("a")
		testutil.Error(t, err, factory.ErrNoCreator,
			"unable to generate resource: no factory creator accepts the resource", map[string]any{"id": "a"})
		testutil.NoError(t, f.Store("b", &resource{id: "b"}))
		testutil.Equal(t, "Stored()", f.Stored(), []string{"b"})
	})
}

// ---------------------------------------------------------------------------
// creators
// ---------------------------------------------------------------------------

func Test_Creators(t *testing.T) {
	t.Run("should try the creators in order until one accepts", func(t *testing.T) {
		first, second, third := &creator{driver: "y"}, &creator{driver: "x"}, &creator{driver: "x"}
		f := newFactory(t, Bag{"a": Bag{"driver": "x"}}, first, second, third)

		_, err := f.Generate("a")
		testutil.NoError(t, err)
		testutil.Equal(t, "accepted", [3]int32{first.accepted.Load(), second.accepted.Load(), third.accepted.Load()}, [3]int32{1, 1, 0})
		testutil.Equal(t, "created", [3]int32{first.created.Load(), second.created.Load(), third.created.Load()}, [3]int32{0, 1, 0})
	})

	t.Run("should ask every creator before failing with ErrNoCreator", func(t *testing.T) {
		first, second := &creator{driver: "x"}, &creator{driver: "y"}
		_, err := newFactory(t, Bag{"a": Bag{"driver": "z"}}, first, second).Generate("a")
		testutil.Error(t, err, factory.ErrNoCreator,
			"unable to generate resource: no factory creator accepts the resource", map[string]any{"id": "a"})
		testutil.Equal(t, "accepted", [2]int32{first.accepted.Load(), second.accepted.Load()}, [2]int32{1, 1})
		testutil.Equal(t, "created", [2]int32{first.created.Load(), second.created.Load()}, [2]int32{0, 0})
	})

	t.Run("should give Accept and Create distinct copies holding the id", func(t *testing.T) {
		var accepted, created flam.IBag
		c := &funcCreator[*resource]{
			accept: func(bag flam.IBag) bool {
				accepted = bag
				ok := bag.String("driver") == "x" && bag.String(factory.ConfigFieldId) == "a"
				testutil.NoError(t, bag.Set("driver", "changed"))
				testutil.NoError(t, bag.Set("n.k", 2))
				return ok
			},
			create: func(bag flam.IBag) (*resource, error) {
				created = bag
				return &resource{id: bag.String(factory.ConfigFieldId)}, nil
			},
		}
		_, err := newFactory(t, Bag{"a": Bag{"driver": "x", "n": Bag{"k": 1}}}, c).Generate("a")
		testutil.NoError(t, err)
		testutil.Equal(t, "Accept bag", accepted, flam.IBag(Bag{"driver": "changed", "id": "a", "n": Bag{"k": 2}}))
		testutil.Equal(t, "Create bag", created, flam.IBag(Bag{"driver": "x", "id": "a", "n": Bag{"k": 1}}))
	})

	t.Run("should replace any configured id field with the resource id", func(t *testing.T) {
		for _, configured := range []any{"other", 1, Bag{"k": "v"}, nil} {
			var got flam.IBag
			c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
				got = bag
				return &resource{id: bag.String(factory.ConfigFieldId)}, nil
			}}
			_, err := newFactory(t, Bag{"a": Bag{factory.ConfigFieldId: configured}}, c).Generate("a")
			testutil.NoError(t, err)
			testutil.Equal(t, fmt.Sprintf("creator bag for %#v", configured), got, flam.IBag(Bag{factory.ConfigFieldId: "a"}))
		}
	})

	t.Run("should let a creator get another resource of its factory", func(t *testing.T) {
		var f flam.IFactory[*resource]
		var dependency *resource
		c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
			id := bag.String(factory.ConfigFieldId)
			if id == "a" {
				var err error
				if dependency, err = f.Get("b"); err != nil {
					return nil, err
				}
			}
			return &resource{id: id}, nil
		}}
		f = newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)

		_, err := f.Get("a")
		testutil.NoError(t, err)
		b, err := f.Get("b")
		testutil.NoError(t, err)
		if dependency != b {
			t.Error("the creator did not get the stored dependency")
		}
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a", "b"})
	})
}

// ---------------------------------------------------------------------------
// Generate and Get failures
// ---------------------------------------------------------------------------

func Test_Creation_Failures(t *testing.T) {
	cause := errors.New("boom")
	scenarios := []struct {
		name     string
		config   Bag
		create   func(flam.IBag) (flam.IResource, error)
		sentinel error
		suffix   string
		ctx      map[string]any
		closed   int32
	}{
		{
			name: "should fail with ErrUnknownID on an id without configuration", config: Bag{"b": Bag{}},
			sentinel: factory.ErrUnknownID, suffix: "unknown factory resource id", ctx: map[string]any{"id": "a"},
		},
		{
			name: "should fail with ErrNoCreator when no creator accepts", config: Bag{"a": Bag{"driver": "other"}},
			sentinel: factory.ErrNoCreator, suffix: "no factory creator accepts the resource", ctx: map[string]any{"id": "a"},
		},
		{
			name: "should wrap the creator error", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return nil, cause },
			sentinel: cause, suffix: "boom", ctx: map[string]any{"id": "a"},
		},
		{
			name: "should fail with ErrNilResource on a nil interface resource", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return nil, nil },
			sentinel: factory.ErrNilResource, suffix: "nil factory resource", ctx: map[string]any{"id": "a"},
		},
		{
			name: "should fail with ErrNilResource on a typed nil resource", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return (*resource)(nil), nil },
			sentinel: factory.ErrNilResource, suffix: "nil factory resource", ctx: map[string]any{"id": "a"},
		},
		{
			name: "should fail with ErrIDMismatch, closing the io.Closer resource", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return &resource{id: "b"}, nil },
			sentinel: factory.ErrIDMismatch, suffix: "factory resource id mismatch",
			ctx: map[string]any{"id": "a", "resource_id": "b"}, closed: 1,
		},
		{
			name: "should fail with ErrIDMismatch on an empty resource id", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return &resource{id: ""}, nil },
			sentinel: factory.ErrIDMismatch, suffix: "factory resource id mismatch",
			ctx: map[string]any{"id": "a", "resource_id": ""}, closed: 1,
		},
		{
			name: "should fail with ErrIDMismatch, without failing on a resource not being an io.Closer", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return &plain{id: "b"}, nil },
			sentinel: factory.ErrIDMismatch, suffix: "factory resource id mismatch",
			ctx: map[string]any{"id": "a", "resource_id": "b"},
		},
		{
			name: "should ignore the Close error of a mismatching resource", config: Bag{"a": Bag{}},
			create:   func(flam.IBag) (flam.IResource, error) { return &resource{id: "b", closeErr: cause}, nil },
			sentinel: factory.ErrIDMismatch, suffix: "factory resource id mismatch",
			ctx: map[string]any{"id": "a", "resource_id": "b"}, closed: 1,
		},
	}

	for _, method := range []string{"Generate", "Get"} {
		t.Run(method, func(t *testing.T) {
			for _, s := range scenarios {
				t.Run(s.name, func(t *testing.T) {
					var created flam.IResource
					c := &funcCreator[flam.IResource]{
						accept: func(bag flam.IBag) bool { return bag.String("driver") == "" },
						create: func(bag flam.IBag) (flam.IResource, error) {
							r, err := s.create(bag)
							created = r
							return r, err
						},
					}
					f := newAnyFactory(t, s.config, c)

					var r flam.IResource
					var err error
					if method == "Get" {
						r, err = f.Get("a")
					} else {
						r, err = f.Generate("a")
					}
					testutil.Error(t, err, s.sentinel, "unable to generate resource: "+s.suffix, s.ctx)
					if r != nil {
						t.Errorf("%s() = %#v, want nil", method, r)
					}
					if closer, ok := created.(*resource); ok && closer != nil {
						testutil.Equal(t, "closed", closer.closed.Load(), s.closed)
					}
					testutil.Equal(t, "Stored()", f.Stored(), []string{})
				})
			}
		})
	}
}

func Test_Generate_Ownership(t *testing.T) {
	t.Run("should not return a stored resource nor store the new one", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{}}, &creator{})
		stored, err := f.Get("a")
		testutil.NoError(t, err)

		generated, err := f.Generate("a")
		testutil.NoError(t, err)
		if generated == stored {
			t.Error("Generate() returned the stored resource")
		}
		got, err := f.Get("a")
		testutil.NoError(t, err)
		if got != stored {
			t.Error("Generate() replaced the stored resource")
		}
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})

		testutil.NoError(t, f.Close())
		testutil.Equal(t, "closed generated", generated.closed.Load(), int32(0))
		testutil.Equal(t, "closed stored", stored.closed.Load(), int32(1))
	})

	t.Run("should fail with ErrUnknownID on an id only stored", func(t *testing.T) {
		f := newFactory(t, Bag{}, &creator{})
		testutil.NoError(t, f.Store("a", &resource{id: "a"}))
		_, err := f.Generate("a")
		testutil.Error(t, err, factory.ErrUnknownID,
			"unable to generate resource: unknown factory resource id", map[string]any{"id": "a"})
	})

	t.Run("should let a creator panic go through", func(t *testing.T) {
		c := &funcCreator[*resource]{create: func(flam.IBag) (*resource, error) { panic("boom") }}
		f := newFactory(t, Bag{"a": Bag{}}, c)
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_, _ = f.Generate("a")
		}()
		testutil.Equal(t, "recovered", recovered, any("boom"))
	})
}

// ---------------------------------------------------------------------------
// Get: storing and concurrency
// ---------------------------------------------------------------------------

func Test_Get_Concurrency(t *testing.T) {
	t.Run("should give the creation error to every concurrent call, storing nothing", func(t *testing.T) {
		release := make(chan struct{})
		c := &creator{create: func(flam.IBag) (*resource, error) {
			<-release
			return nil, errors.New("boom")
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		errs := make([]error, 5)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Go(func() { _, errs[i] = f.Get("a") })
		}
		waitForWaitingGets(t, len(errs)-1)
		close(release)
		wg.Wait()

		testutil.Equal(t, "created", c.created.Load(), int32(1))
		for _, err := range errs {
			testutil.Error(t, err, err, "unable to generate resource: boom", map[string]any{"id": "a"})
		}
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should retry the creation after a creator panic", func(t *testing.T) {
		panics := true
		c := &creator{create: func(bag flam.IBag) (*resource, error) {
			if panics {
				panic("boom")
			}
			return &resource{id: "a"}, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_, _ = f.Get("a")
		}()
		testutil.Equal(t, "recovered", recovered, any("boom"))
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
		testutil.NoError(t, f.Store("b", &resource{id: "b"})) // not deadlocked

		panics = false
		r, err := f.Get("a")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", r.Id(), "a")
		testutil.Equal(t, "created", c.created.Load(), int32(2))
	})

	t.Run("should refuse to Store under an id being created by Get", func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		created := &resource{id: "a"}
		c := &creator{create: func(flam.IBag) (*resource, error) {
			close(started)
			<-release
			return created, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		done := make(chan *resource)
		go func() {
			r, err := f.Get("a")
			if err != nil {
				t.Error(err)
			}
			done <- r
		}()
		<-started

		refused := &resource{id: "a"}
		testutil.Error(t, f.Store("a", refused), factory.ErrDuplicateID,
			"unable to store resource: factory resource id already in use", map[string]any{"id": "a"})
		testutil.Equal(t, "Stored() while creating", f.Stored(), []string{})
		close(release)

		if <-done != created {
			t.Error("Get() did not return the created resource")
		}
		testutil.Equal(t, "refused closed", refused.closed.Load(), int32(0))
	})

	t.Run("should fail the calls waiting for a resource created across Close", func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		r := &resource{id: "a"}
		c := &creator{create: func(flam.IBag) (*resource, error) {
			close(started)
			<-release
			return r, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}}, c)

		errs := make([]error, 3)
		var wg sync.WaitGroup
		wg.Go(func() { _, errs[0] = f.Get("a") })
		<-started
		for i := 1; i < len(errs); i++ {
			wg.Go(func() { _, errs[i] = f.Get("a") })
		}
		waitForWaitingGets(t, len(errs)-1)
		testutil.NoError(t, f.Close())
		close(release)
		wg.Wait()

		for _, err := range errs {
			testutil.Error(t, err, factory.ErrClosed, "unable to get resource: factory closed", map[string]any{"id": "a"})
		}
		testutil.Equal(t, "closed", r.closed.Load(), int32(1))
		testutil.Equal(t, "Stored()", f.Stored(), []string(nil))
	})

	t.Run("should not fail on a resource not being an io.Closer created across Close", func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		c := &funcCreator[flam.IResource]{create: func(flam.IBag) (flam.IResource, error) {
			close(started)
			<-release
			return &plain{id: "a"}, nil
		}}
		f := newAnyFactory(t, Bag{"a": Bag{}}, c)

		done := make(chan error)
		go func() {
			_, err := f.Get("a")
			done <- err
		}()
		<-started
		testutil.NoError(t, f.Close())
		close(release)
		testutil.Error(t, <-done, factory.ErrClosed, "unable to get resource: factory closed", map[string]any{"id": "a"})
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		config := Bag{}
		for i := range 8 {
			config[fmt.Sprintf("r%d", i)] = Bag{}
		}
		f := newFactory(t, config, &creator{})

		var wg sync.WaitGroup
		for i := range 8 {
			id := fmt.Sprintf("r%d", i)
			wg.Go(func() {
				for range 50 {
					_, _ = f.Get(id)
					_ = f.Has(id)
					_ = f.Available()
					_ = f.Stored()
					if r, err := f.Generate(id); err == nil {
						_ = r.Close()
					}
					_ = f.Remove(id)
					_ = f.Store(id, &resource{id: id})
				}
			})
		}
		wg.Go(func() {
			for range 50 {
				_ = f.GenerateAll()
				_ = f.RemoveAll()
			}
		})
		wg.Wait()
		testutil.NoError(t, f.Close())
		testutil.Equal(t, "Stored()", f.Stored(), []string(nil))
	})
}

// ---------------------------------------------------------------------------
// Store: resources given by the caller
// ---------------------------------------------------------------------------

func Test_Store_Rules(t *testing.T) {
	t.Run("should fail on a nil resource of an interface type", func(t *testing.T) {
		f := newAnyFactory(t, Bag{})
		for name, r := range map[string]flam.IResource{"nil": nil, "typed nil": (*resource)(nil), "typed nil plain": (*plain)(nil)} {
			testutil.Error(t, f.Store("a", r), factory.ErrNilResource,
				"unable to store resource: nil factory resource", map[string]any{"id": "a"})
			testutil.Equal(t, "Stored() after "+name, f.Stored(), []string{})
		}
	})

	t.Run("should store under an available id, then Get returns it without creating", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}}, c)
		r := &resource{id: "a"}
		testutil.NoError(t, f.Store("a", r))

		got, err := f.Get("a")
		testutil.NoError(t, err)
		if got != r {
			t.Error("Get() did not return the stored resource")
		}
		testutil.Equal(t, "created", c.created.Load(), int32(0))
	})

	t.Run("should store a resource not being an io.Closer", func(t *testing.T) {
		f := newAnyFactory(t, Bag{})
		r := &plain{id: "a"}
		testutil.NoError(t, f.Store("a", r))
		got, err := f.Get("a")
		testutil.NoError(t, err)
		if got != flam.IResource(r) {
			t.Error("Get() did not return the stored resource")
		}
	})

	t.Run("should store under the empty id", func(t *testing.T) {
		f := newFactory(t, Bag{})
		testutil.NoError(t, f.Store("", &resource{id: ""}))
		testutil.Equal(t, "Stored()", f.Stored(), []string{""})
		testutil.Equal(t, "Has(\"\")", f.Has(""), true)
	})

	t.Run("should leave a refused duplicate to the caller, keeping the stored one", func(t *testing.T) {
		f := newFactory(t, Bag{})
		stored, refused := &resource{id: "a"}, &resource{id: "a"}
		testutil.NoError(t, f.Store("a", stored))
		testutil.Error(t, f.Store("a", refused), factory.ErrDuplicateID,
			"unable to store resource: factory resource id already in use", map[string]any{"id": "a"})

		got, err := f.Get("a")
		testutil.NoError(t, err)
		if got != stored {
			t.Error("Store() replaced the stored resource")
		}
		testutil.Equal(t, "refused closed", refused.closed.Load(), int32(0))
	})

	t.Run("should refuse an id used by a resource created by Get", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{}}, &creator{})
		_, err := f.Get("a")
		testutil.NoError(t, err)
		testutil.Error(t, f.Store("a", &resource{id: "a"}), factory.ErrDuplicateID,
			"unable to store resource: factory resource id already in use", map[string]any{"id": "a"})
	})

	t.Run("should store again under an id once removed", func(t *testing.T) {
		f := newFactory(t, Bag{})
		testutil.NoError(t, f.Store("a", &resource{id: "a"}))
		testutil.NoError(t, f.Remove("a"))
		testutil.NoError(t, f.Store("a", &resource{id: "a"}))
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})
	})
}

// ---------------------------------------------------------------------------
// Lookup
// ---------------------------------------------------------------------------

func Test_Lookup(t *testing.T) {
	t.Run("should return the stored resources, without creating any", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)
		stored := &resource{id: "s"}
		testutil.NoError(t, f.Store("s", stored))
		created, err := f.Get("a")
		testutil.NoError(t, err)

		for id, want := range map[string]*resource{"s": stored, "a": created} {
			got, ok := f.Lookup(id)
			if !ok || got != want {
				t.Errorf("Lookup(%s) = %v, %v, want the stored resource", id, got, ok)
			}
		}
		for _, id := range []string{"b", "x"} {
			if got, ok := f.Lookup(id); ok || got != nil {
				t.Errorf("Lookup(%s) = %v, %v, want nil, false", id, got, ok)
			}
		}
		testutil.Equal(t, "created", c.created.Load(), int32(1))
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a", "s"})
	})

	t.Run("should not return a removed resource", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{}}, &creator{})
		_, err := f.Get("a")
		testutil.NoError(t, err)
		testutil.NoError(t, f.Remove("a"))
		if _, ok := f.Lookup("a"); ok {
			t.Error("Lookup() found a removed resource")
		}
	})
}

// ---------------------------------------------------------------------------
// Available, Stored, Has
// ---------------------------------------------------------------------------

func Test_Listing(t *testing.T) {
	t.Run("should return sorted ids", func(t *testing.T) {
		f := newFactory(t, Bag{"c": Bag{}, "a": Bag{}, "b": Bag{}, "B": Bag{}}, &creator{})
		testutil.Equal(t, "Available()", f.Available(), []string{"B", "a", "b", "c"})

		for _, id := range []string{"z", "x", "y"} {
			testutil.NoError(t, f.Store(id, &resource{id: id}))
		}
		_, err := f.Get("b")
		testutil.NoError(t, err)
		testutil.Equal(t, "Stored()", f.Stored(), []string{"b", "x", "y", "z"})
		testutil.Equal(t, "Available()", f.Available(), []string{"B", "a", "b", "c"})
	})

	t.Run("should return empty, non-nil, slices on an empty open factory", func(t *testing.T) {
		f := newFactory(t, nil)
		testutil.Equal(t, "Available()", f.Available(), []string{})
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should return new slices on each call", func(t *testing.T) {
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}})
		testutil.NoError(t, f.Store("a", &resource{id: "a"}))
		available, stored := f.Available(), f.Stored()
		available[0], stored[0] = "changed", "changed"
		testutil.Equal(t, "Available()", f.Available(), []string{"a", "b"})
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})
	})

	t.Run("should tell whether Get can return a resource", func(t *testing.T) {
		f := newFactory(t, Bag{"configured": Bag{}, "both": Bag{}}, &creator{})
		testutil.NoError(t, f.Store("stored", &resource{id: "stored"}))
		_, err := f.Get("both")
		testutil.NoError(t, err)

		for id, want := range map[string]bool{"configured": true, "both": true, "stored": true, "unknown": false, "": false} {
			testutil.Equal(t, "Has("+id+")", f.Has(id), want)
		}

		testutil.NoError(t, f.Remove("stored"))
		testutil.Equal(t, "Has(stored) after Remove", f.Has("stored"), false)
		testutil.NoError(t, f.Remove("both"))
		testutil.Equal(t, "Has(both) after Remove", f.Has("both"), true)
	})
}

// ---------------------------------------------------------------------------
// GenerateAll
// ---------------------------------------------------------------------------

func Test_GenerateAll_Rules(t *testing.T) {
	t.Run("should create the resources in id order", func(t *testing.T) {
		var order []string
		c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
			id := bag.String(factory.ConfigFieldId)
			order = append(order, id)
			return &resource{id: id}, nil
		}}
		f := newFactory(t, Bag{"c": Bag{}, "a": Bag{}, "b": Bag{}}, c)
		testutil.NoError(t, f.GenerateAll())
		testutil.Equal(t, "order", order, []string{"a", "b", "c"})
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a", "b", "c"})
	})

	t.Run("should succeed without configuration", func(t *testing.T) {
		f := newFactory(t, Bag{})
		testutil.NoError(t, f.GenerateAll())
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should keep the resources stored under available ids", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)
		r := &resource{id: "a"}
		testutil.NoError(t, f.Store("a", r))
		testutil.NoError(t, f.GenerateAll())

		got, err := f.Get("a")
		testutil.NoError(t, err)
		if got != r {
			t.Error("GenerateAll() replaced the stored resource")
		}
		testutil.Equal(t, "created", c.created.Load(), int32(1))
	})

	t.Run("should return the Get error, wrapping the creator one, keeping the resources created before", func(t *testing.T) {
		cause := errors.New("boom")
		var order []string
		c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
			id := bag.String(factory.ConfigFieldId)
			order = append(order, id)
			if id == "b" {
				return nil, cause
			}
			return &resource{id: id}, nil
		}}
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}, "c": Bag{}}, c)
		testutil.Error(t, f.GenerateAll(), cause, "unable to generate resource: boom", map[string]any{"id": "b"})
		testutil.Equal(t, "order", order, []string{"a", "b"})
		testutil.Equal(t, "Stored()", f.Stored(), []string{"a"})
	})

	t.Run("should fail with ErrClosed, without id, when a creator closes the factory", func(t *testing.T) {
		var f flam.IFactory[*resource]
		c := &funcCreator[*resource]{create: func(bag flam.IBag) (*resource, error) {
			testutil.NoError(t, f.Close())
			return &resource{id: bag.String(factory.ConfigFieldId)}, nil
		}}
		f = newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)
		testutil.Error(t, f.GenerateAll(), factory.ErrClosed,
			"unable to generate resources: factory closed", map[string]any{})
	})
}

// ---------------------------------------------------------------------------
// Remove, RemoveAll, Close: releasing resources
// ---------------------------------------------------------------------------

func Test_Remove_Rules(t *testing.T) {
	t.Run("should remove a resource not being an io.Closer without failing", func(t *testing.T) {
		c := &funcCreator[flam.IResource]{create: func(bag flam.IBag) (flam.IResource, error) {
			return &plain{id: bag.String(factory.ConfigFieldId)}, nil
		}}
		f := newAnyFactory(t, Bag{"a": Bag{}}, c)
		r1, err := f.Get("a")
		testutil.NoError(t, err)

		testutil.NoError(t, f.Remove("a"))
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
		r2, err := f.Get("a")
		testutil.NoError(t, err)
		if r1 == r2 {
			t.Error("Get() returned the removed resource")
		}
	})

	t.Run("should forget a resource stored without configuration", func(t *testing.T) {
		f := newFactory(t, Bag{})
		r := &resource{id: "a"}
		testutil.NoError(t, f.Store("a", r))
		testutil.NoError(t, f.Remove("a"))
		testutil.Equal(t, "closed", r.closed.Load(), int32(1))
		testutil.Equal(t, "Has(a)", f.Has("a"), false)
		_, err := f.Get("a")
		testutil.Error(t, err, factory.ErrUnknownID,
			"unable to generate resource: unknown factory resource id", map[string]any{"id": "a"})
	})

	t.Run("should fail with ErrUnknownID on an id removed already, closing once", func(t *testing.T) {
		f := newFactory(t, Bag{})
		r := &resource{id: "a"}
		testutil.NoError(t, f.Store("a", r))
		testutil.NoError(t, f.Remove("a"))
		testutil.Error(t, f.Remove("a"), factory.ErrUnknownID,
			"unable to remove resource: unknown factory resource id", map[string]any{"id": "a"})
		testutil.Equal(t, "closed", r.closed.Load(), int32(1))
	})

	t.Run("should fail with ErrUnknownID on an unknown id", func(t *testing.T) {
		testutil.Error(t, newFactory(t, Bag{}).Remove("x"), factory.ErrUnknownID,
			"unable to remove resource: unknown factory resource id", map[string]any{"id": "x"})
	})
}

// releaseScenarios are the methods removing every stored resource, with
// the factory state they leave.
var releaseScenarios = []struct {
	name    string
	release func(f flam.IFactory[*resource]) error
	stored  []string
}{
	{"RemoveAll", func(f flam.IFactory[*resource]) error { return f.RemoveAll() }, []string{}},
	{"Close", func(f flam.IFactory[*resource]) error { return f.Close() }, nil},
}

func Test_Release(t *testing.T) {
	cause1, cause2 := errors.New("boom1"), errors.New("boom2")

	for _, s := range releaseScenarios {
		t.Run(s.name, func(t *testing.T) {
			t.Run("should close the resources in id order", func(t *testing.T) {
				var order []string
				f := newFactory(t, Bag{})
				for _, id := range []string{"c", "a", "b"} {
					testutil.NoError(t, f.Store(id, &resource{id: id, order: &order}))
				}
				testutil.NoError(t, s.release(f))
				testutil.Equal(t, "order", order, []string{"a", "b", "c"})
				testutil.Equal(t, "Stored()", f.Stored(), s.stored)
			})

			t.Run("should return nil without stored resources", func(t *testing.T) {
				testutil.NoError(t, s.release(newFactory(t, Bag{"a": Bag{}})))
			})

			t.Run("should not fail on resources not being io.Closer", func(t *testing.T) {
				f := newAnyFactory(t, Bag{})
				closer := &resource{id: "b"}
				testutil.NoError(t, f.Store("a", &plain{id: "a"}))
				testutil.NoError(t, f.Store("b", closer))
				var err error
				if s.name == "Close" {
					err = f.Close()
				} else {
					err = f.RemoveAll()
				}
				testutil.NoError(t, err)
				testutil.Equal(t, "closed", closer.closed.Load(), int32(1))
			})

			t.Run("should join the Close errors, each an *Error holding the id", func(t *testing.T) {
				f := newFactory(t, Bag{})
				a, b, c := &resource{id: "a", closeErr: cause1}, &resource{id: "b"}, &resource{id: "c", closeErr: cause2}
				for _, r := range []*resource{c, b, a} {
					testutil.NoError(t, f.Store(r.id, r))
				}

				err := s.release(f)
				errs := joinedErrors(t, err)
				if len(errs) != 2 {
					t.Fatalf("joined %d errors, want 2", len(errs))
				}
				testutil.Error(t, errs[0], cause1, "unable to remove resource: boom1", map[string]any{"id": "a"})
				testutil.Error(t, errs[1], cause2, "unable to remove resource: boom2", map[string]any{"id": "c"})
				testutil.Equal(t, "Error()", err.Error(), "unable to remove resource: boom1\nunable to remove resource: boom2")
				testutil.Equal(t, "closed", [3]int32{a.closed.Load(), b.closed.Load(), c.closed.Load()}, [3]int32{1, 1, 1})
				testutil.Equal(t, "Stored()", f.Stored(), s.stored)
			})

			t.Run("should join a single Close error too", func(t *testing.T) {
				f := newFactory(t, Bag{})
				testutil.NoError(t, f.Store("a", &resource{id: "a", closeErr: cause1}))
				errs := joinedErrors(t, s.release(f))
				if len(errs) != 1 {
					t.Fatalf("joined %d errors, want 1", len(errs))
				}
				testutil.Error(t, errs[0], cause1, "unable to remove resource: boom1", map[string]any{"id": "a"})
			})

			t.Run("should not close the resources a second time", func(t *testing.T) {
				f := newFactory(t, Bag{})
				r := &resource{id: "a", closeErr: cause1}
				testutil.NoError(t, f.Store("a", r))
				_ = s.release(f)
				if s.name == "Close" {
					testutil.NoError(t, f.Close())
				} else {
					testutil.NoError(t, f.RemoveAll())
				}
				testutil.Equal(t, "closed", r.closed.Load(), int32(1))
			})
		})
	}

	t.Run("RemoveAll should keep the configurations and the factory open", func(t *testing.T) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}}, c)
		r1, err := f.Get("a")
		testutil.NoError(t, err)
		testutil.NoError(t, f.RemoveAll())

		testutil.Equal(t, "Available()", f.Available(), []string{"a"})
		r2, err := f.Get("a")
		testutil.NoError(t, err)
		if r1 == r2 {
			t.Error("Get() returned the removed resource")
		}
		testutil.NoError(t, f.Store("b", &resource{id: "b"}))
	})
}

// ---------------------------------------------------------------------------
// closed factory
// ---------------------------------------------------------------------------

func Test_Closed(t *testing.T) {
	newClosed := func(t *testing.T) (flam.IFactory[*resource], *creator) {
		c := &creator{}
		f := newFactory(t, Bag{"a": Bag{}, "b": Bag{}}, c)
		testutil.NoError(t, f.Store("s", &resource{id: "s"}))
		_, err := f.Get("a")
		testutil.NoError(t, err)
		testutil.NoError(t, f.Close())
		return f, c
	}

	t.Run("should report nothing", func(t *testing.T) {
		f, _ := newClosed(t)
		testutil.Equal(t, "Available()", f.Available(), []string(nil))
		testutil.Equal(t, "Stored()", f.Stored(), []string(nil))
		for _, id := range []string{"a", "b", "s", "x"} {
			testutil.Equal(t, "Has("+id+")", f.Has(id), false)
		}
	})

	scenarios := []struct {
		name string
		call func(f flam.IFactory[*resource], id string) error
		text string
	}{
		{"Get", func(f flam.IFactory[*resource], id string) error { _, err := f.Get(id); return err }, "unable to get resource: factory closed"},
		{"Generate", func(f flam.IFactory[*resource], id string) error { _, err := f.Generate(id); return err }, "unable to generate resource: factory closed"},
		{"Store", func(f flam.IFactory[*resource], id string) error { return f.Store(id, &resource{id: id}) }, "unable to store resource: factory closed"},
		{"Remove", func(f flam.IFactory[*resource], id string) error { return f.Remove(id) }, "unable to remove resource: factory closed"},
	}
	for _, s := range scenarios {
		t.Run("should fail "+s.name+" with ErrClosed, for any id, without creating", func(t *testing.T) {
			f, c := newClosed(t)
			for _, id := range []string{"a", "b", "s", "x"} {
				testutil.Error(t, s.call(f, id), factory.ErrClosed, s.text, map[string]any{"id": id})
			}
			testutil.Equal(t, "created", c.created.Load(), int32(1))
		})
	}

	t.Run("should fail GenerateAll and RemoveAll with ErrClosed, without id", func(t *testing.T) {
		f, c := newClosed(t)
		testutil.Error(t, f.GenerateAll(), factory.ErrClosed, "unable to generate resources: factory closed", map[string]any{})
		testutil.Error(t, f.RemoveAll(), factory.ErrClosed, "unable to remove resources: factory closed", map[string]any{})
		testutil.Equal(t, "created", c.created.Load(), int32(1))
	})

	t.Run("should fail Store with ErrClosed before checking the resource", func(t *testing.T) {
		f, _ := newClosed(t)
		for _, r := range []*resource{nil, {id: "b"}} {
			testutil.Error(t, f.Store("a", r), factory.ErrClosed,
				"unable to store resource: factory closed", map[string]any{"id": "a"})
		}
	})

	t.Run("should fail Store with ErrClosed when closed while checking the resource", func(t *testing.T) {
		f := newAnyFactory(t, Bag{})
		r := &closingResource{id: "a", close: func() { _ = f.Close() }}
		testutil.Error(t, f.Store("a", r), factory.ErrClosed,
			"unable to store resource: factory closed", map[string]any{"id": "a"})
		testutil.Equal(t, "Stored()", f.Stored(), []string(nil))
	})

	t.Run("should look up nothing", func(t *testing.T) {
		f, _ := newClosed(t)
		for _, id := range []string{"a", "s"} {
			if _, ok := f.Lookup(id); ok {
				t.Errorf("Lookup(%s) found a resource", id)
			}
		}
	})

	t.Run("should leave a refused resource to the caller", func(t *testing.T) {
		f, _ := newClosed(t)
		r := &resource{id: "a"}
		_ = f.Store("a", r)
		testutil.Equal(t, "closed", r.closed.Load(), int32(0))
	})

	t.Run("should return nil on Close again, even after Close errors", func(t *testing.T) {
		f := newFactory(t, Bag{})
		testutil.NoError(t, f.Store("a", &resource{id: "a", closeErr: errors.New("boom")}))
		if f.Close() == nil {
			t.Fatal("Close() = nil, want the resource Close error")
		}
		for range 2 {
			testutil.NoError(t, f.Close())
		}
	})

	t.Run("should close an empty factory", func(t *testing.T) {
		f := newFactory(t, nil)
		testutil.NoError(t, f.Close())
		testutil.Equal(t, "Available()", f.Available(), []string(nil))
	})
}

// ---------------------------------------------------------------------------
// contract
// ---------------------------------------------------------------------------

func Test_Contract(t *testing.T) {
	t.Run("should return *flam.Error values from every method but Close and RemoveAll", func(t *testing.T) {
		f := newFactory(t, Bag{})
		_, err := f.Get("x")
		var e *flam.Error
		for name, err := range map[string]error{
			"Get":      err,
			"Store":    f.Store("x", nil),
			"Remove":   f.Remove("x"),
			"Generate": func() error { _, err := f.Generate("x"); return err }(),
		} {
			if !errors.As(err, &e) || e != err {
				t.Errorf("%s() error = %T, want a *flam.Error", name, err)
			}
		}
	})

	t.Run("should have the documented sentinel messages", func(t *testing.T) {
		for sentinel, text := range map[error]string{
			factory.ErrNilCreator:   "nil factory creator",
			factory.ErrClosed:       "factory closed",
			factory.ErrUnknownID:    "unknown factory resource id",
			factory.ErrDuplicateID:  "factory resource id already in use",
			factory.ErrNoCreator:    "no factory creator accepts the resource",
			factory.ErrNilResource:  "nil factory resource",
			factory.ErrIDMismatch:   "factory resource id mismatch",
			factory.ErrCreatorPanic: "factory creator panicked",
		} {
			testutil.Equal(t, "Error()", sentinel.Error(), text)
		}
		testutil.Equal(t, "ConfigFieldDriver", factory.ConfigFieldDriver, "driver")
	})
}

// closingResource is a flam.IResource calling close from its Id method,
// such as to close its factory while Store checks it.
type closingResource struct {
	id    string
	close func()
}

func (r *closingResource) Id() string {
	r.close()
	return r.id
}
