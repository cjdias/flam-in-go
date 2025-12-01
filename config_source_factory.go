package flam

import (
	"io"
	"sort"

	"go.uber.org/dig"
)

type ConfigSourceFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (ConfigSource, error)
	Store(id string, source ConfigSource) error
	Remove(id string) error
	RemoveAll() error

	SetPriority(id string, priority int) error
	Reload() error
}

type configSources []ConfigSource

func (container configSources) Len() int {
	return len(container)
}

func (container configSources) Swap(
	i,
	j int,
) {
	container[i], container[j] = container[j], container[i]
}

func (container configSources) Less(
	i,
	j int,
) bool {
	return container[i].GetPriority() < container[j].GetPriority()
}

type configSourceFactory struct {
	factory *factory[ConfigSource]
	config  *config
}

var _ ConfigSourceFactory = (*configSourceFactory)(nil)

type configSourceFactoryArgs struct {
	dig.In

	Creators      []ConfigSourceCreator `group:"flam.config.sources.creator"`
	FactoryConfig FactoryConfig
	Config        *config
}

func newConfigSourceFactory(
	args configSourceFactoryArgs,
) (ConfigSourceFactory, error) {
	var creators []FactoryResourceCreator[ConfigSource]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	f, _ := NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("ConfigSource"),
		PathConfigSources)

	return &configSourceFactory{
		factory: f.(*factory[ConfigSource]),
		config:  args.Config}, nil
}

func (factory configSourceFactory) Close() error {
	return factory.factory.Close()
}

func (factory configSourceFactory) Available() []string {
	return factory.factory.Available()
}

func (factory configSourceFactory) Stored() []string {
	return factory.factory.Stored()
}

func (factory configSourceFactory) Has(
	id string,
) bool {
	return factory.factory.Has(id)
}

func (factory configSourceFactory) Get(
	id string,
) (ConfigSource, error) {
	source, e := factory.factory.Get(id)
	if e == nil {
		factory.reload()
	}

	return source, e
}

func (factory configSourceFactory) Store(
	id string,
	value ConfigSource,
) error {
	e := factory.factory.Store(id, value)
	if e == nil {
		factory.reload()
	}

	return e
}

func (factory configSourceFactory) Remove(
	id string,
) error {
	e := factory.factory.Remove(id)
	if e == nil {
		factory.reload()
	}

	return e
}

func (factory configSourceFactory) RemoveAll() error {
	e := factory.factory.RemoveAll()
	if e == nil {
		factory.reload()
	}

	return e
}

func (factory configSourceFactory) Reload() error {
	factory.factory.locker.Lock()

	reloaded := false
	for _, source := range factory.factory.entries {
		if observable, ok := source.(ObservableConfigSource); ok {
			updated, e := observable.Reload()
			if e != nil {
				factory.factory.locker.Unlock()
				return e
			}

			reloaded = reloaded || updated
		}
	}
	factory.factory.locker.Unlock()

	if reloaded {
		factory.reload()
	}

	return nil
}

func (factory configSourceFactory) SetPriority(
	id string,
	priority int,
) error {
	factory.factory.locker.Lock()
	source, ok := factory.factory.entries[id]
	if !ok {
		factory.factory.locker.Unlock()
		return newErrUnknownResource("ConfigSource", id)
	}
	source.SetPriority(priority)
	factory.factory.locker.Unlock()

	factory.reload()

	return nil
}

func (factory configSourceFactory) reload() {
	factory.factory.locker.Lock()
	defer factory.factory.locker.Unlock()

	sources := configSources{}
	for _, source := range factory.factory.entries {
		sources = append(sources, source)
	}
	sort.Sort(sources)

	data := Bag{}
	for _, source := range sources {
		sourceData := source.Get("", Bag{})
		data.Merge(sourceData.(Bag))
	}

	factory.config.locker.Lock()
	factory.config.sourcesBag = data
	factory.config.locker.Unlock()

	factory.config.rebuild()
}
