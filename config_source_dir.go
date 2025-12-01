package flam

import (
	"os"
	"sync"
)

type dirConfigSource struct {
	configSource

	disk         Disk
	path         string
	configParser ConfigParser
	recursive    bool
}

var _ ConfigSource = (*dirConfigSource)(nil)

func newDirConfigSource(
	priority int,
	disk Disk,
	path string,
	configParser ConfigParser,
	recursive bool,
) (ConfigSource, error) {
	source := &dirConfigSource{
		configSource: configSource{
			mutex:    &sync.Mutex{},
			bag:      Bag{},
			priority: priority},
		disk:         disk,
		path:         path,
		configParser: configParser,
		recursive:    recursive}

	if e := source.load(); e != nil {
		return nil, e
	}

	return source, nil
}

func (source *dirConfigSource) load() error {
	bag, e := source.loadDir(source.path)
	if e != nil {
		return e
	}

	source.mutex.Lock()
	source.bag = bag
	source.mutex.Unlock()

	return nil
}

func (source *dirConfigSource) loadDir(
	path string,
) (Bag, error) {
	dir, e := source.disk.Open(path)
	if e != nil {
		return nil, e
	}
	defer func() { _ = dir.Close() }()

	files, e := dir.Readdir(0)
	if e != nil {
		return nil, e
	}

	loaded := Bag{}
	for _, file := range files {
		if file.IsDir() {
			if source.recursive {
				partial, e := source.loadDir(path + "/" + file.Name())
				if e != nil {
					return nil, e
				}
				loaded.Merge(partial)
			}
		} else {
			partial, e := source.loadFile(path + "/" + file.Name())
			if e != nil {
				return nil, e
			}
			loaded.Merge(partial)
		}
	}

	return loaded, nil
}

func (source *dirConfigSource) loadFile(
	path string,
) (Bag, error) {
	file, e := source.disk.OpenFile(path, os.O_RDONLY, 0o644)
	if e != nil {
		return nil, e
	}
	defer func() { _ = file.Close() }()

	return source.configParser.Parse(file)
}
