package env

import (
	"maps"
	"slices"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/static"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IConfigSourceCreator] of the env sources,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.IConfigSourceCreator {
	return &creator{config: config}
}

// creator is the [flam.IConfigSourceCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

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

	files, ok := creatorFiles(c.field(bag, ConfigFieldFiles, DefaultFilesConfigPath, nil))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldFiles)
	}

	mappings, err := creatorMappings(id, c.field(bag, ConfigFieldMappings, DefaultMappingsConfigPath, nil))
	if err != nil {
		return nil, err
	}

	source, err := newSource(id, priority, files, mappings)
	if err != nil || reload {
		return source, err
	}
	return static.New(source), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create env config source"

// field returns the value of the field of the source configuration, or
// else its default.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return base.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorFiles returns the files of a list of strings, a []string or a
// []any holding strings only, or of nil, meaning no file.
func creatorFiles(
	value any,
) ([]string, bool) {
	switch v := value.(type) {
	case nil:
		return nil, true
	case []string:
		return slices.Clone(v), true
	case []any:
		files := make([]string, 0, len(v))
		for _, item := range v {
			file, ok := item.(string)
			if !ok {
				return nil, false
			}
			files = append(files, file)
		}
		return files, true
	}
	return nil, false
}

// creatorMappings returns the mappings of a bag, a container.Bag or a
// map[string]any, or of nil, meaning no mapping. It checks that each
// mapping has a variable name and a valid path, and that no two of them
// write over each other. The checks look at the configuration only, so
// that a conflict is reported whether or not the variables are set.
func creatorMappings(
	id string,
	value any,
) (map[string]string, error) {
	var bag container.Bag
	switch v := value.(type) {
	case nil:
	case container.Bag:
		bag = v
	case map[string]any:
		bag = v
	default:
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldMappings)
	}

	mappings := make(map[string]string, len(bag))
	for key, path := range bag.All() {
		str, isString := path.(string)
		if key == "" || !isString || !container.ValidPath(str) || str == "" {
			return nil, newErrInvalidMapping(creatorCreateMessage, id, key, path)
		}
		mappings[key] = str
	}

	keys := slices.Sorted(maps.Keys(mappings))
	for i, key := range keys {
		for _, other := range keys[i+1:] {
			if creatorPathsOverlap(mappings[key], mappings[other]) {
				return nil, newErrMappingConflict(creatorCreateMessage, id,
					key, mappings[key], other, mappings[other])
			}
		}
	}
	return mappings, nil
}

// creatorPathsOverlap reports whether one path is the other, or lies
// inside it.
func creatorPathsOverlap(
	path string,
	other string,
) bool {
	return path == other ||
		strings.HasPrefix(path, other+container.BagPathSeparator) ||
		strings.HasPrefix(other, path+container.BagPathSeparator)
}
