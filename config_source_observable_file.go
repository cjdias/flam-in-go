package flam

import (
	"sync"
	"time"
)

type observableFileConfigSource struct {
	fileConfigSource

	timer     Timer
	timestamp time.Time
}

var _ ConfigSource = (*observableFileConfigSource)(nil)
var _ ObservableConfigSource = (*observableFileConfigSource)(nil)

func newObservableFileConfigSource(
	priority int,
	disk Disk,
	path string,
	configParser ConfigParser,
	timer Timer,
) (ObservableConfigSource, error) {
	source := &observableFileConfigSource{
		fileConfigSource: fileConfigSource{
			configSource: configSource{
				mutex:    &sync.Mutex{},
				bag:      Bag{},
				priority: priority},
			disk:         disk,
			path:         path,
			configParser: configParser},
		timer:     timer,
		timestamp: timer.Unix(0, 0)}

	if _, e := source.Reload(); e != nil {
		return nil, e
	}

	return source, nil
}

func (source *observableFileConfigSource) Reload() (bool, error) {
	fileStats, e := source.disk.Stat(source.path)
	if e != nil {
		return false, e
	}

	modTime := fileStats.ModTime()
	if source.timestamp.Equal(source.timer.Unix(0, 0)) || source.timestamp.Before(modTime) {
		if e := source.load(); e != nil {
			return false, e
		}
		source.timestamp = modTime

		return true, nil
	}

	return false, nil
}
