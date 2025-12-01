package flam

import (
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
)

type FactoryResource any

type FactoryResourceCreator[R FactoryResource] interface {
	Accept(config Bag) bool
	Create(config Bag) (R, error)
}

type FactoryConfigValidator func(id string, config Bag) error

func DriverFactoryConfigValidator(
	resourceName string,
) FactoryConfigValidator {
	return func(id string, config Bag) error {
		if config.String("driver") == "" {
			return newErrInvalidResourceConfig(resourceName, id, config)
		}

		return nil
	}
}

type Factory[R FactoryResource] interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (R, error)
	Store(id string, value R) error
	Generate(id string) (R, error)
	GenerateAll() error
	Remove(id string) error
	RemoveAll() error
}

type factory[R FactoryResource] struct {
	locker                 *sync.Mutex
	creators               []FactoryResourceCreator[R]
	factoryConfig          FactoryConfig
	factoryConfigValidator FactoryConfigValidator
	factoryConfigPath      string
	entries                map[string]R
}

var _ Factory[string] = (*factory[string])(nil)

func NewFactory[R FactoryResource](
	creators []FactoryResourceCreator[R],
	factoryConfig FactoryConfig,
	factoryConfigValidator FactoryConfigValidator,
	factoryConfigPath string,
) (Factory[R], error) {
	if factoryConfig == nil {
		return nil, newErrNilReference("config")
	}

	return &factory[R]{
		locker:                 &sync.Mutex{},
		creators:               creators,
		factoryConfig:          factoryConfig,
		factoryConfigValidator: factoryConfigValidator,
		factoryConfigPath:      factoryConfigPath,
		entries:                map[string]R{}}, nil
}

func (factory *factory[R]) Close() error {
	factory.locker.Lock()
	defer factory.locker.Unlock()

	for _, entry := range factory.entries {
		if closer, ok := any(entry).(io.Closer); ok {
			if e := closer.Close(); e != nil {
				return e
			}
		}
	}

	factory.entries = map[string]R{}

	return nil
}

func (factory *factory[R]) Available() []string {
	factory.locker.Lock()
	defer factory.locker.Unlock()

	factoryConfig := factory.factoryConfig.Get(factory.factoryConfigPath)
	ids := factoryConfig.Entries()

	for k := range factory.entries {
		if !slices.Contains(ids, k) {
			ids = append(ids, k)
		}
	}

	slices.SortFunc(ids, strings.Compare)

	return ids
}

func (factory *factory[R]) Stored() []string {
	factory.locker.Lock()
	defer factory.locker.Unlock()

	var ids []string
	for k := range factory.entries {
		ids = append(ids, k)
	}

	slices.SortFunc(ids, strings.Compare)

	return ids
}

func (factory *factory[R]) Has(
	id string,
) bool {
	_, ok := slices.BinarySearch(factory.Available(), id)

	return ok
}

func (factory *factory[R]) Get(
	id string,
) (R, error) {
	factory.locker.Lock()
	if entry, ok := factory.entries[id]; ok {
		factory.locker.Unlock()
		return entry, nil
	}
	factory.locker.Unlock()

	return factory.Generate(id)
}

func (factory *factory[R]) Store(
	id string,
	resource R,
) error {
	switch {
	case any(resource) == nil,
		reflect.ValueOf(resource).IsNil():
		return newErrNilReference("resource")
	case factory.Has(id):
		return newErrDuplicateResource(id)
	}

	factory.locker.Lock()
	defer factory.locker.Unlock()

	factory.entries[id] = resource

	return nil
}

func (factory *factory[R]) Generate(
	id string,
) (R, error) {
	factory.locker.Lock()
	defer factory.locker.Unlock()

	var zero R
	factoryConfig := factory.factoryConfig.Get(factory.factoryConfigPath)
	config := factoryConfig.Bag(id)
	if config == nil {
		return zero, newErrUnknownResource(reflect.TypeFor[R]().Name(), id)
	}
	_ = config.Set("id", id)

	if factory.factoryConfigValidator != nil {
		if e := factory.factoryConfigValidator(id, config); e != nil {
			return zero, e
		}
	}

	for _, creator := range factory.creators {
		if creator.Accept(config) {
			entry, e := creator.Create(config)
			if e != nil {
				return zero, e
			}

			factory.entries[id] = entry

			return entry, nil
		}
	}

	return zero, newErrUnacceptedResourceConfig(reflect.TypeFor[R]().Name(), config)
}

func (factory *factory[R]) GenerateAll() error {
	factoryConfig := factory.factoryConfig.Get(factory.factoryConfigPath)
	ids := factoryConfig.Entries()
	for _, id := range ids {
		if _, ok := factory.entries[id]; ok {
			continue
		}

		if _, e := factory.Generate(id); e != nil {
			return e
		}
	}

	return nil
}

func (factory *factory[R]) Remove(
	id string,
) error {
	factory.locker.Lock()
	defer factory.locker.Unlock()

	entry, ok := factory.entries[id]
	if !ok {
		return newErrUnknownResource(reflect.TypeFor[R]().Name(), id)
	}

	if closer, ok := any(entry).(io.Closer); ok {
		if e := closer.Close(); e != nil {
			return e
		}
	}

	delete(factory.entries, id)

	return nil
}

func (factory *factory[R]) RemoveAll() error {
	factory.locker.Lock()
	defer factory.locker.Unlock()

	for _, entry := range factory.entries {
		if closer, ok := any(entry).(io.Closer); ok {
			if e := closer.Close(); e != nil {
				return e
			}
		}
	}

	factory.entries = map[string]R{}

	return nil
}
