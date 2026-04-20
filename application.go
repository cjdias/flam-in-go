package flam

import (
	"slices"
	"sync"

	"go.uber.org/dig"
)

type Application interface {
	Container() *dig.Container
	HasProvider(id string) bool
	Register(provider Provider) error
	Boot() error
	Run() error
	Close() error
}

type application struct {
	mu        sync.Mutex
	config    Bag
	container *dig.Container
	providers []Provider
	isBooted  bool
}

var _ Application = (*application)(nil)

func NewApplication(
	config ...Bag,
) Application {
	app := &application{
		config:    append(config, Bag{})[0],
		container: dig.New(),
		providers: []Provider{},
		isBooted:  false}

	_ = app.Register(newProvider())

	return app
}

func (app *application) Container() *dig.Container {
	return app.container
}

func (app *application) HasProvider(id string) bool {
	app.mu.Lock()
	defer app.mu.Unlock()

	for _, registered := range app.providers {
		if registered.Id() == id {
			return true
		}
	}

	return false
}

func (app *application) Register(
	provider Provider,
) error {
	if provider == nil {
		return newErrNilReference("provider")
	}

	app.mu.Lock()
	defer app.mu.Unlock()

	for _, registered := range app.providers {
		if registered.Id() == provider.Id() {
			return newErrDuplicateProvider(provider.Id())
		}
	}

	if e := provider.Register(app.container); e != nil {
		return e
	}

	app.providers = append(app.providers, provider)

	return nil
}

func (app *application) Boot() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.isBooted {
		return nil
	}

	config := &Bag{}
	for _, provider := range app.providers {
		if configurable, ok := provider.(ConfigurableProvider); ok {
			if e := configurable.Config(config); e != nil {
				return e
			}
		}
	}
	config.Merge(app.config)

	if e := app.container.Invoke(func(factory ConfigSourceFactory) error {
		source := &configSource{
			mu:       sync.Mutex{},
			bag:      *config,
			priority: DefaultConfigPriority}
		return factory.Store(DefaultConfigSourceId, source)
	}); e != nil {
		return e
	}

	for _, provider := range app.providers {
		if bootable, ok := provider.(BootableProvider); ok {
			if e := bootable.Boot(app.container); e != nil {
				return e
			}
		}
	}

	app.isBooted = true

	return nil
}

func (app *application) Run() error {
	if !app.isBooted {
		if e := app.Boot(); e != nil {
			return e
		}
	}

	app.mu.Lock()
	defer app.mu.Unlock()

	for _, provider := range app.providers {
		if runnable, ok := provider.(RunnableProvider); ok {
			if e := runnable.Run(app.container); e != nil {
				return e
			}
		}
	}

	return nil
}

func (app *application) Close() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	slices.Reverse(app.providers)
	var errors []error
	for _, provider := range app.providers {
		if closable, ok := provider.(ClosableProvider); ok {
			if e := closable.Close(app.container); e != nil {
				errors = append(errors, e)
			}
		}
	}
	if len(errors) > 0 {
		return newErrPublishFailed(errors)
	}

	return nil
}
