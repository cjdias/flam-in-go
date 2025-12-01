package flam

import (
	"io"
	"sync"
)

type ConfigSource interface {
	io.Closer

	GetPriority() int
	SetPriority(priority int)
	Get(path string, def ...any) any
}

type ObservableConfigSource interface {
	ConfigSource

	Reload() (bool, error)
}

type configSource struct {
	mutex    sync.Locker
	bag      Bag
	priority int
}

func (*configSource) Close() error {
	return nil
}

func (source *configSource) GetPriority() int {
	return source.priority
}

func (source *configSource) SetPriority(
	priority int,
) {
	source.priority = priority
}

func (source *configSource) Get(
	path string,
	def ...any,
) any {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	return source.bag.Get(path, def...)
}
