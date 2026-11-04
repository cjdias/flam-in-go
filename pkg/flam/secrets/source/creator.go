package source

import (
	"maps"
	"slices"
	"strings"
	"sync"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The application is taken instead of the store factory, so
// that the factory is created, and reads the configuration of its stores,
// only when a source needs it, once the sources sorting before it were
// added to the configuration.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	App    flam.IApp
}

// newCreator returns the [flam.IConfigSourceCreator] of the secrets
// sources, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IConfigSourceCreator {
	return &creator{config: args.Config, app: args.App}
}

// creator is the [flam.IConfigSourceCreator] of the configurations with
// the [Driver] driver, while mu guards the store factory, taken from the
// container once.
type creator struct {
	config flam.IConfig
	app    flam.IApp

	mu     sync.Mutex
	stores flam.ISecretStoreFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the source of the fields of the configuration, or of
// their defaults, holding the secrets of its values, read from its store.
// A source without values takes no store.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IConfigSource, error) {
	id := bag.String(factory.ConfigFieldId)

	priority, ok := c.field(bag, ConfigFieldPriority, DefaultPriorityConfigPath, defaultPriority).(int)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPriority)
	}
	reload, ok := c.field(bag, ConfigFieldReload, DefaultReloadConfigPath, defaultReload).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldReload)
	}
	storeID, ok := c.field(bag, ConfigFieldStore, DefaultStoreConfigPath, defaultStore).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldStore)
	}
	values, err := creatorValues(id, c.field(bag, ConfigFieldValues, DefaultValuesConfigPath, nil))
	if err != nil {
		return nil, err
	}

	s := &source{id: id, priority: priority, values: values, bag: container.Bag{}}
	if len(values) != 0 {
		if storeID == "" {
			return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldStore)
		}
		stores, err := c.factory()
		if err != nil {
			return nil, newErr(ErrMissingFactory, creatorCreateMessage, id, err)
		}
		if s.store, err = stores.Get(storeID); err != nil {
			return nil, newErr(ErrStore, creatorCreateMessage, id, err).
				With(ErrorFieldStore, storeID)
		}
		if s.bag, err = s.load(creatorCreateMessage); err != nil {
			return nil, err
		}
	}
	if !reload {
		return &staticSource{IConfigSource: s}, nil
	}
	return s, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create secrets config source"

// field returns the value of the field of the source configuration, or
// else its default.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// factory returns the store factory, taken from the container on the first
// call that succeeds.
func (c *creator) factory() (flam.ISecretStoreFactory, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stores == nil {
		if err := c.app.Invoke(func(f flam.ISecretStoreFactory) { c.stores = f }); err != nil {
			return nil, err
		}
	}
	return c.stores, nil
}

// creatorValues returns the keys of the values of a bag, a container.Bag or
// a map[string]any, or of nil, meaning no value, by configuration path:
// each string held by the bag, at any depth, is the key of the path made of
// the keys leading to it. It checks that each key is a non-empty string,
// each path valid, and that no path lies inside another one.
func creatorValues(
	id string,
	value any,
) (map[string]string, error) {
	bag, ok := creatorBag(value)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldValues)
	}
	values := map[string]string{}
	if err := creatorFlatten(id, "", bag, values); err != nil {
		return nil, err
	}

	paths := slices.Sorted(maps.Keys(values))
	for i, path := range paths {
		for _, other := range paths[i+1:] {
			if strings.HasPrefix(other, path+container.BagPathSeparator) {
				return nil, newErrInvalidValue(creatorCreateMessage, id, other)
			}
		}
	}
	return values, nil
}

// creatorFlatten adds to values the keys of the bag, whose path is prefix,
// by the paths of their values.
func creatorFlatten(
	id string,
	prefix string,
	bag container.Bag,
	values map[string]string,
) error {
	for name, value := range bag.All() {
		path := name
		if prefix != "" {
			path = prefix + container.BagPathSeparator + name
		}
		if name == "" || !container.ValidPath(path) {
			return newErrInvalidValue(creatorCreateMessage, id, path)
		}
		if nested, ok := creatorBag(value); ok && value != nil {
			if err := creatorFlatten(id, path, nested, values); err != nil {
				return err
			}
			continue
		}
		key, ok := value.(string)
		if !ok || key == "" {
			return newErrInvalidValue(creatorCreateMessage, id, path)
		}
		values[path] = key
	}
	return nil
}

// creatorBag returns the bag of a container.Bag or a map[string]any, or of
// nil, an empty bag, and whether the value is one of them.
func creatorBag(
	value any,
) (container.Bag, bool) {
	switch v := value.(type) {
	case nil:
		return container.Bag{}, true
	case container.Bag:
		return v, true
	case map[string]any:
		return v, true
	}
	return nil, false
}
