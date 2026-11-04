package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	"go.yaml.in/yaml/v3"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// newParser returns a YAML parserImpl of the given id.
func newParser(
	id string,
) flam.IConfigParser {
	return &parserImpl{id: id}
}

// parserImpl is the YAML [flam.IConfigParser]. It holds no state but its id,
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

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil {
		if errors.Is(err, io.EOF) {
			return container.Bag{}, nil // no document
		}
		return nil, newErrInvalidDocument(parserParseMessage, p.id, err)
	}
	if err := decoder.Decode(&yaml.Node{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected document after the first one")
		}
		return nil, newErrInvalidDocument(parserParseMessage, p.id, err)
	}

	parserRetag(&node)
	var document any
	if err := node.Decode(&document); err != nil {
		return nil, newErrInvalidDocument(parserParseMessage, p.id, err)
	}

	switch value := parserNormalize(document).(type) {
	case nil:
		return container.Bag{}, nil
	case container.Bag:
		return value, nil
	}
	return nil, newErrNotMapping(parserParseMessage, p.id)
}

// parserParseMessage is the message of the errors returned by Parse.
const parserParseMessage = "unable to parse yaml config"

// parserRetag tags as strings the scalar keys of the mappings, but the
// merge keys, so that they decode to the text written, and the timestamps,
// which would otherwise decode to time.Time values. Aliases are not
// followed: their anchored nodes are in the tree.
func parserRetag(
	node *yaml.Node,
) {
	if node.Kind == yaml.ScalarNode && node.ShortTag() == "!!timestamp" {
		node.Tag = "!!str"
	}
	for i, child := range node.Content {
		isKey := node.Kind == yaml.MappingNode && i%2 == 0
		if isKey && child.Kind == yaml.ScalarNode && child.ShortTag() != "!!merge" {
			child.Tag = "!!str"
		}
		parserRetag(child)
	}
}

// parserNormalize converts a decoded value: mappings become container.Bag
// values, and integers float64 values when they do not fit in an int, the
// library decoding the other ones as int values.
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
	case map[any]any: // a key aliasing a scalar that is not a string
		bag := make(container.Bag, len(v))
		for key, item := range v {
			bag[fmt.Sprint(key)] = parserNormalize(item)
		}
		return bag
	case []any:
		for i, item := range v {
			v[i] = parserNormalize(item)
		}
		return v
	case int64, uint64: // an int64 only on 32 bit platforms
		return reflect.ValueOf(v).Convert(reflect.TypeFor[float64]()).Float()
	}
	return value
}
