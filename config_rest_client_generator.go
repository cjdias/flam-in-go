package flam

import (
	"net/http"
)

type ConfigRestClientGenerator interface {
	Create() (ConfigRestClient, error)
}

type ConfigRestClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type configRestClientGenerator struct{}

var _ ConfigRestClientGenerator = (*configRestClientGenerator)(nil)

func newConfigRestClientGenerator() ConfigRestClientGenerator {
	return &configRestClientGenerator{}
}

func (configRestClientGenerator) Create() (ConfigRestClient, error) {
	return &http.Client{}, nil
}
