package flam

import (
	"net/http"
	"sync"
)

type restConfigSource struct {
	configSource

	configRestClient ConfigRestClient
	uri              string
	configParser     ConfigParser
	configPath       string
}

var _ ConfigSource = (*restConfigSource)(nil)

func newRestConfigSource(
	priority int,
	configRestClient ConfigRestClient,
	uri string,
	configParser ConfigParser,
	configPath string,
) (ConfigSource, error) {
	source := &restConfigSource{
		configSource: configSource{
			mutex:    &sync.Mutex{},
			bag:      Bag{},
			priority: priority},
		configRestClient: configRestClient,
		uri:              uri,
		configParser:     configParser,
		configPath:       configPath}

	if e := source.load(); e != nil {
		return nil, e
	}

	return source, nil
}

func (source *restConfigSource) load() error {
	response, e := source.request()
	if e != nil {
		return e
	}

	bag, e := source.getConfig(response)
	if e != nil {
		return e
	}

	source.mutex.Lock()
	source.bag = bag
	source.mutex.Unlock()

	return nil
}

func (source *restConfigSource) request() (Bag, error) {
	request, e := http.NewRequest(http.MethodGet, source.uri, http.NoBody)
	if e != nil {
		return nil, e
	}

	response, e := source.configRestClient.Do(request)
	if e != nil {
		return nil, e
	}

	return source.configParser.Parse(response.Body)
}

func (source *restConfigSource) getConfig(
	response Bag,
) (Bag, error) {
	config := response.Get(source.configPath)
	if config == nil {
		return Bag{}, newErrRestConfigSourceConfigNotFound(source.configPath, response)
	}

	bag, ok := config.(Bag)
	if !ok {
		return Bag{}, newErrInvalidRestConfigSourceConfig(source.configPath, config)
	}

	return bag, nil
}
