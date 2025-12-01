package flam

import (
	"sync"
	"time"
)

type observableRestConfigSource struct {
	restConfigSource

	timestampPath string
	timestamp     time.Time
	timer         Timer
}

var _ ConfigSource = (*observableRestConfigSource)(nil)
var _ ObservableConfigSource = (*observableRestConfigSource)(nil)

func newObservableRestConfigSource(
	priority int,
	configRestClient ConfigRestClient,
	uri string,
	configParser ConfigParser,
	configPath string,
	timestampPath string,
	timer Timer,
) (ConfigSource, error) {
	source := &observableRestConfigSource{
		restConfigSource: restConfigSource{
			configSource: configSource{
				mutex:    &sync.Mutex{},
				bag:      Bag{},
				priority: priority},
			uri:              uri,
			configPath:       configPath,
			configRestClient: configRestClient,
			configParser:     configParser},
		timestampPath: timestampPath,
		timestamp:     timer.Now(),
		timer:         timer}

	if _, e := source.Reload(); e != nil {
		return nil, e
	}

	return source, nil
}

func (source *observableRestConfigSource) Reload() (bool, error) {
	response, e := source.request()
	if e != nil {
		return false, e
	}

	timestamp, e := source.getTimestamp(response)
	if e != nil {
		return false, e
	}

	if source.timestamp.Equal(source.timer.Unix(0, 0)) || source.timestamp.Before(timestamp) {
		bag, e := source.getConfig(response)
		if e != nil {
			return false, e
		}

		source.mutex.Lock()
		source.bag = bag
		source.timestamp = timestamp
		source.mutex.Unlock()

		return true, nil
	}

	return false, nil
}

func (source *observableRestConfigSource) getTimestamp(
	response Bag,
) (time.Time, error) {
	timestamp := response.Get(source.timestampPath)
	if timestamp == nil {
		return time.Unix(0, 0), newErrRestConfigSourceTimestampNotFound(source.timestampPath, response)
	}

	stringTimestamp, ok := timestamp.(string)
	if !ok {
		return time.Unix(0, 0), newErrInvalidRestConfigSourceTimestamp(source.timestampPath, timestamp)
	}

	parsedTimestamp, e := source.timer.Parse(time.RFC3339, stringTimestamp)
	if e != nil {
		return time.Unix(0, 0), e
	}

	return parsedTimestamp, nil
}
