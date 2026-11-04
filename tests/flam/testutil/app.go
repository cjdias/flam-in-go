package testutil

import (
	"fmt"
	"sync"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// App is a fake flam.IApp recording its calls, as "kind/options" such as
// "provider/1", or "bundle:id", and delegating Provide, Invoke and Decorate
// to a real dig.Container. Fail, when set, is called first by
// RegisterBundle, Provide, Invoke and Decorate with the call kind, and may
// return an error to fail the call with, or panic.
type App struct {
	Fail func(kind string) error

	mu        sync.Mutex
	container *dig.Container
	calls     []string
	bundles   map[string]bool
}

// compile time check that *App implements flam.IApp
var _ flam.IApp = (*App)(nil)

// NewApp returns an App with an empty container.
func NewApp() *App {
	return &App{container: dig.New(), bundles: map[string]bool{}}
}

// Calls returns a copy of the calls recorded so far.
func (a *App) Calls() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.calls...)
}

// Close does nothing and returns nil.
func (a *App) Close() error { return nil }

// HasBundle reports whether a bundle with the id was registered.
func (a *App) HasBundle(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.bundles[id]
}

// RegisterBundle records the call, marks the bundle registered and calls
// its Register method, without calling Config, nor checking for
// duplicates.
func (a *App) RegisterBundle(bundle flam.IBundle) error {
	if err := a.record("bundle:"+bundle.Id(), "bundle"); err != nil {
		return err
	}
	a.mu.Lock()
	a.bundles[bundle.Id()] = true
	a.mu.Unlock()
	return bundle.Register(a)
}

// Container returns the dig.Container the calls are delegated to.
func (a *App) Container() *dig.Container { return a.container }

// Invoke records the call and delegates it to the container.
func (a *App) Invoke(invoker any, opts ...dig.InvokeOption) error {
	if err := a.record(fmt.Sprintf("invoker/%d", len(opts)), "invoker"); err != nil {
		return err
	}
	return a.container.Invoke(invoker, opts...)
}

// Provide records the call and delegates it to the container.
func (a *App) Provide(provider any, opts ...dig.ProvideOption) error {
	if err := a.record(fmt.Sprintf("provider/%d", len(opts)), "provider"); err != nil {
		return err
	}
	return a.container.Provide(provider, opts...)
}

// Decorate records the call and delegates it to the container.
func (a *App) Decorate(decorator any, opts ...dig.DecorateOption) error {
	if err := a.record(fmt.Sprintf("decorator/%d", len(opts)), "decorator"); err != nil {
		return err
	}
	return a.container.Decorate(decorator, opts...)
}

// Boot does nothing and returns nil.
func (a *App) Boot() error { return nil }

// Run does nothing and returns nil.
func (a *App) Run() error { return nil }

// record records a call, then returns the Fail error for its kind.
func (a *App) record(call, kind string) error {
	a.mu.Lock()
	a.calls = append(a.calls, call)
	a.mu.Unlock()
	if a.Fail != nil {
		return a.Fail(kind)
	}
	return nil
}
