package checks

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health/checks"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// newApp returns a testutil.App with the config bundle, a fake clock, the
// bundles given, and a configuration holding their defaults, then the
// values, at paths.
func newApp(t *testing.T, values map[string]any, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	all := append([]flam.IBundle{config.NewBundle()}, bundles...)

	bag := container.Bag{}
	for _, b := range all {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	a := testutil.NewApp()
	clock := testutil.NewClock(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	testutil.NoError(t, a.Provide(func() flam.IClock { return clock }))
	for _, b := range all {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// creatorsArgs are the creators of the check creator group.
type creatorsArgs struct {
	dig.In

	Creators []flam.IHealthCheckCreator `group:"flam.health.checks.creator"`
}

// creators returns the creators of the check creator group of the app.
func creators(t *testing.T, a flam.IApp) []flam.IHealthCheckCreator {
	t.Helper()
	var all []flam.IHealthCheckCreator
	testutil.NoError(t, a.Invoke(func(args creatorsArgs) { all = args.Creators }))
	return all
}

// creatorOf returns the single creator of the check creator group of the
// app.
func creatorOf(t *testing.T, a flam.IApp) flam.IHealthCheckCreator {
	t.Helper()
	all := creators(t, a)
	if len(all) != 1 {
		t.Fatalf("creators = %d, want 1", len(all))
	}
	return all[0]
}

// checkBag returns the configuration of the check of the id, with the
// driver and the fields.
func checkBag(id string, driver string, fields container.Bag) flam.IBag {
	bag := container.Bag{factory.ConfigFieldId: id, factory.ConfigFieldDriver: driver}
	for field, value := range fields {
		bag[field] = value
	}
	return bag
}

// wantErr fails the test unless err wraps the sentinels and its outermost
// *flam.Error holds exactly the context.
func wantErr(t *testing.T, err error, ctx map[string]any, sentinels ...error) {
	t.Helper()
	for _, sentinel := range sentinels {
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
		}
	}
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T, want a *flam.Error", err)
	}
	testutil.Context(t, e, ctx)
}

func Test_Bundle(t *testing.T) {
	b := checks.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks")

	a := newApp(t, nil, b)
	testutil.Equal(t, "calls", a.Calls()[len(a.Calls())-14:], []string{
		"bundle:flam.health.checks",
		"bundle:flam.health.checks.database", "provider/1",
		"bundle:flam.health.checks.redis", "provider/1",
		"bundle:flam.health.checks.kennel", "provider/1",
		"bundle:flam.health.checks.broker", "provider/1",
		"bundle:flam.health.checks.http", "provider/1",
		"bundle:flam.health.checks.disk", "provider/1",
		"invoker/0",
	})
	testutil.Equal(t, "creators", len(creators(t, a)), 6)
}
