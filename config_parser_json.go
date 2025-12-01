package flam

import (
	"encoding/json"
	"io"
)

type jsonConfigParser struct{}

var _ ConfigParser = (*jsonConfigParser)(nil)

func newJsonConfigParser() ConfigParser {
	return &jsonConfigParser{}
}

func (parser jsonConfigParser) Close() error {
	return nil
}

func (parser jsonConfigParser) Parse(
	reader io.Reader,
) (Bag, error) {
	b, e := io.ReadAll(reader)
	if e != nil {
		return nil, e
	}

	data := map[string]any{}
	if e := json.Unmarshal(b, &data); e != nil {
		return nil, e
	}

	return BagNormalization(data).(Bag), nil
}
