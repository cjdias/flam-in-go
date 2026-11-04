package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// newParser returns a JSON parserImpl of the given id.
func newParser(
	id string,
) flam.IConfigParser {
	return &parserImpl{id: id}
}

// parserImpl is the JSON [flam.IConfigParser]. It holds no state but its id,
// so it is safe for concurrent use and has nothing to close.
type parserImpl struct {
	id string
}

// compile time check that *parserImpl implements IConfigParser
var _ flam.IConfigParser = (*parserImpl)(nil)

func (p *parserImpl) Id() string {
	return p.id
}

func (p *parserImpl) Close() error {
	return nil
}

func (p *parserImpl) Parse(
	reader io.Reader,
) (flam.IBag, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, newErrRead(parserParseMessage, p.id, err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return container.Bag{}, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, newErrInvalidDocument(parserParseMessage, p.id, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected data after the document")
		}
		return nil, newErrInvalidDocument(parserParseMessage, p.id, err)
	}

	switch value := parserNormalize(document).(type) {
	case nil:
		return container.Bag{}, nil
	case container.Bag:
		return value, nil
	}
	return nil, newErrNotObject(parserParseMessage, p.id)
}

// parserParseMessage is the message of the errors returned by Parse.
const parserParseMessage = "unable to parse json config"

// parserNormalize converts a value decoded with UseNumber: objects become
// container.Bag values, and numbers int values when whole and fitting in an
// int, or float64 values otherwise.
func parserNormalize(
	value any,
) any {
	switch v := value.(type) {
	case map[string]any:
		bag := make(container.Bag, len(v))
		for key, item := range v {
			bag[key] = parserNormalize(item)
		}
		return bag
	case []any:
		for i, item := range v {
			v[i] = parserNormalize(item)
		}
		return v
	case json.Number:
		if i, err := v.Int64(); err == nil && i >= math.MinInt && i <= math.MaxInt {
			return int(i)
		}
		f, _ := v.Float64() // a valid JSON number, possibly rounded to ±Inf
		return f
	}
	return value
}
