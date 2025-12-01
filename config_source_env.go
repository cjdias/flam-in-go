package flam

import (
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type envConfigSource struct {
	configSource

	files    []string
	mappings map[string]string
}

var _ ConfigSource = (*envConfigSource)(nil)

func newEnvConfigSource(
	priority int,
	files []string,
	mappings map[string]string,
) (ConfigSource, error) {
	source := &envConfigSource{
		configSource: configSource{
			mutex:    &sync.Mutex{},
			bag:      Bag{},
			priority: priority},
		files:    files,
		mappings: mappings}

	if e := source.load(); e != nil {
		return nil, e
	}

	return source, nil
}

func (source *envConfigSource) load() error {
	if len(source.files) != 0 {
		if e := godotenv.Load(source.files...); e != nil {
			return e
		}
	}

	for key, path := range source.mappings {
		env := os.Getenv(key)
		if env == "" {
			continue
		}

		step := source.bag
		sections := strings.Split(path, ".")
		for i, section := range sections {
			if i != len(sections)-1 {
				if _, ok := step[section]; !ok {
					step[section] = Bag{}
				}
				step = step[section].(Bag)
			} else {
				step[section] = env
			}
		}
	}

	return nil
}
