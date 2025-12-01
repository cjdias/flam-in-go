package flam

import (
	"os"
	"sync"
)

type fileConfigSource struct {
	configSource

	disk         Disk
	path         string
	configParser ConfigParser
}

var _ ConfigSource = (*fileConfigSource)(nil)

func newFileConfigSource(
	priority int,
	disk Disk,
	path string,
	configParser ConfigParser,
) (ConfigSource, error) {
	source := &fileConfigSource{
		configSource: configSource{
			mutex:    &sync.Mutex{},
			bag:      Bag{},
			priority: priority},
		disk:         disk,
		path:         path,
		configParser: configParser}

	if e := source.load(); e != nil {
		return nil, e
	}

	return source, nil
}

func (source *fileConfigSource) load() error {
	file, e := source.disk.OpenFile(source.path, os.O_RDONLY, 0o644)
	if e != nil {
		return e
	}
	defer func() { _ = file.Close() }()

	bag, e := source.configParser.Parse(file)
	if e != nil {
		return e
	}

	source.mutex.Lock()
	defer source.mutex.Unlock()

	source.bag = bag

	return nil
}
