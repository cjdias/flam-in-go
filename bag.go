package flam

import (
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

type Bag map[string]any

func (bag *Bag) Clone() Bag {
	var cloner func(value any) any
	cloner = func(value any) any {
		switch typedValue := value.(type) {
		case []any:
			var result []any
			for _, i := range typedValue {
				result = append(result, cloner(i))
			}

			return result
		case Bag:
			return typedValue.Clone()
		case *Bag:
			return typedValue.Clone()
		default:
			return value
		}
	}

	target := Bag{}
	for key, value := range *bag {
		target[key] = cloner(value)
	}

	return target
}

func (bag *Bag) Entries() []string {
	var result []string
	for key := range *bag {
		result = append(result, key)
	}

	return result
}

func (bag *Bag) Has(
	path string,
) bool {
	_, e := bag.path(path)

	return e == nil
}

func (bag *Bag) Get(
	path string,
	def ...any,
) any {
	val, e := bag.path(path)
	if e != nil {
		return append(def, nil)[0]
	}

	return val
}

func (bag *Bag) Bool(
	path string,
	def ...bool,
) bool {
	if val, ok := bag.Get(path).(bool); ok {
		return val
	}

	return append(def, false)[0]
}

func (bag *Bag) Int(
	path string,
	def ...int,
) int {
	if val, ok := bag.Get(path).(int); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Int8(
	path string,
	def ...int8,
) int8 {
	if val, ok := bag.Get(path).(int8); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Int16(
	path string,
	def ...int16,
) int16 {
	if val, ok := bag.Get(path).(int16); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Int32(
	path string,
	def ...int32,
) int32 {
	if val, ok := bag.Get(path).(int32); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Int64(
	path string,
	def ...int64,
) int64 {
	if val, ok := bag.Get(path).(int64); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Uint(
	path string,
	def ...uint,
) uint {
	if val, ok := bag.Get(path).(uint); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Uint8(
	path string,
	def ...uint8,
) uint8 {
	if val, ok := bag.Get(path).(uint8); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Uint16(
	path string,
	def ...uint16,
) uint16 {
	if val, ok := bag.Get(path).(uint16); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Uint32(
	path string,
	def ...uint32,
) uint32 {
	if val, ok := bag.Get(path).(uint32); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Uint64(
	path string,
	def ...uint64,
) uint64 {
	if val, ok := bag.Get(path).(uint64); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Float32(
	path string,
	def ...float32,
) float32 {
	if val, ok := bag.Get(path).(float32); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) Float64(
	path string,
	def ...float64,
) float64 {
	if val, ok := bag.Get(path).(float64); ok {
		return val
	}

	return append(def, 0)[0]
}

func (bag *Bag) String(
	path string,
	def ...string,
) string {
	if val, ok := bag.Get(path).(string); ok {
		return val
	}

	return append(def, "")[0]
}

func (bag *Bag) StringMap(
	path string,
	def ...map[string]any,
) map[string]any {
	if val, ok := bag.Get(path).(map[string]any); ok {
		return val
	}

	return append(def, nil)[0]
}

func (bag *Bag) StringMapString(
	path string,
	def ...map[string]string,
) map[string]string {
	if val, ok := bag.Get(path).(map[string]string); ok {
		return val
	}

	return append(def, nil)[0]
}

func (bag *Bag) Slice(
	path string,
	def ...[]any,
) []any {
	if val, ok := bag.Get(path).([]any); ok {
		return val
	}

	return append(def, nil)[0]
}

func (bag *Bag) StringSlice(
	path string,
	def ...[]string,
) []string {
	if val, ok := bag.Get(path).([]string); ok {
		return val
	}

	return append(def, nil)[0]
}

func (bag *Bag) Duration(
	path string,
	def ...time.Duration,
) time.Duration {
	switch tval := bag.Get(path).(type) {
	case int:
		return time.Duration(tval) * time.Millisecond
	case int64:
		return time.Duration(tval) * time.Millisecond
	case time.Duration:
		return tval
	default:
		if len(def) != 0 {
			return def[0]
		}
	}

	return time.Duration(0)
}

func (bag *Bag) Bag(
	path string,
	def ...Bag,
) Bag {
	if val, ok := bag.Get(path).(Bag); ok {
		return val
	}

	return append(def, nil)[0]
}

func (bag *Bag) Set(
	path string,
	value any,
) error {
	if path == "" {
		return newErrBagInvalidPath("")
	}

	parts := strings.Split(path, ".")
	it := bag
	if len(parts) == 1 {
		(*it)[path] = value
		return nil
	}

	generate := func(part string) {
		generate := false
		if next, ok := (*it)[part]; !ok {
			generate = true
		} else if _, ok = next.(Bag); !ok {
			generate = true
		}
		if generate {
			(*it)[part] = Bag{}
		}
	}

	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			continue
		}

		generate(part)
		next := (*it)[part].(Bag)
		it = &next
	}

	part := parts[len(parts)-1:][0]
	generate(part)
	(*it)[part] = value

	return nil
}

func (bag *Bag) Merge(
	src Bag,
) *Bag {
	for key, value := range src {
		switch tValue := value.(type) {
		case Bag:
			switch tLocal := (*bag)[key].(type) {
			case Bag:
				tLocal.Merge(tValue)
			case *Bag:
				tLocal.Merge(tValue)
			default:
				v := Bag{}
				v.Merge(tValue)
				(*bag)[key] = v
			}
		case *Bag:
			switch tLocal := (*bag)[key].(type) {
			case Bag:
				tLocal.Merge(*tValue)
			case *Bag:
				tLocal.Merge(*tValue)
			default:
				v := Bag{}
				v.Merge(*tValue)
				(*bag)[key] = v
			}
		default:
			(*bag)[key] = value
		}
	}

	return bag
}

func (bag *Bag) Populate(
	target any,
	path ...string,
) error {
	p := append(path, "")[0]
	source := bag.Get(p, nil)
	if source == nil {
		return newErrBagInvalidPath(p)
	}

	return mapstructure.Decode(source, target)
}

func (bag *Bag) path(
	path string,
) (any, error) {
	var ok bool
	var it any

	it = *bag
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}

		switch typedIt := it.(type) {
		case Bag:
			if it, ok = typedIt[part]; !ok {
				return nil, newErrBagInvalidPath(path)
			}
		case *Bag:
			if it, ok = (*typedIt)[part]; !ok {
				return nil, newErrBagInvalidPath(path)
			}
		default:
			return nil, newErrBagInvalidPath(path)
		}
	}

	return it, nil
}

// -----------------------------------------------------------------------------

func BagNormalization(
	val any,
) any {
	if pValue, ok := val.(Bag); ok {
		result := Bag{}
		for k, value := range pValue {
			result[strings.ToLower(k)] = BagNormalization(value)
		}

		return result
	}

	if lValue, ok := val.([]any); ok {
		var result []any
		for _, i := range lValue {
			result = append(result, BagNormalization(i))
		}

		return result
	}

	if mValue, ok := val.(map[string]any); ok {
		result := Bag{}
		for k, i := range mValue {
			result[strings.ToLower(k)] = BagNormalization(i)
		}

		return result
	}

	if mValue, ok := val.(map[any]any); ok {
		result := Bag{}
		for k, i := range mValue {
			stringKey, ok := k.(string)
			if ok {
				result[strings.ToLower(stringKey)] = BagNormalization(i)
			} else {
				result[fmt.Sprintf("%v", k)] = BagNormalization(i)
			}
		}

		return result
	}

	if fValue, ok := val.(float64); ok && float64(int(fValue)) == fValue {
		return int(fValue)
	}

	return val
}
