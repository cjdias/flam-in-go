package flam

import (
	"io"

	"gopkg.in/yaml.v3"
)

type yamlConfigParser struct{}

var _ ConfigParser = (*yamlConfigParser)(nil)

func newYamlConfigParser() ConfigParser {
	return &yamlConfigParser{}
}

func (parser yamlConfigParser) Close() error {
	return nil
}

func (parser yamlConfigParser) Parse(
	reader io.Reader,
) (Bag, error) {
	b, e := io.ReadAll(reader)
	if e != nil {
		return nil, e
	}

	data := map[string]any{}
	if e := yaml.Unmarshal(b, &data); e != nil {
		return nil, e
	}

	return BagNormalization(data).(Bag), nil
}
