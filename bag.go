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
		default:
			if b, ok := asBag(typedValue); ok {
				return b.Clone()
			}
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
	return get(bag, path, def...)
}

func (bag *Bag) Int(
	path string,
	def ...int,
) int {
	return get(bag, path, def...)
}

func (bag *Bag) Int8(
	path string,
	def ...int8,
) int8 {
	return get(bag, path, def...)
}

func (bag *Bag) Int16(
	path string,
	def ...int16,
) int16 {
	return get(bag, path, def...)
}

func (bag *Bag) Int32(
	path string,
	def ...int32,
) int32 {
	return get(bag, path, def...)
}

func (bag *Bag) Int64(
	path string,
	def ...int64,
) int64 {
	return get(bag, path, def...)
}

func (bag *Bag) Uint(
	path string,
	def ...uint,
) uint {
	return get(bag, path, def...)
}

func (bag *Bag) Uint8(
	path string,
	def ...uint8,
) uint8 {
	return get(bag, path, def...)
}

func (bag *Bag) Uint16(
	path string,
	def ...uint16,
) uint16 {
	return get(bag, path, def...)
}

func (bag *Bag) Uint32(
	path string,
	def ...uint32,
) uint32 {
	return get(bag, path, def...)
}

func (bag *Bag) Uint64(
	path string,
	def ...uint64,
) uint64 {
	return get(bag, path, def...)
}

func (bag *Bag) Float32(
	path string,
	def ...float32,
) float32 {
	return get(bag, path, def...)
}

func (bag *Bag) Float64(
	path string,
	def ...float64,
) float64 {
	return get(bag, path, def...)
}

func (bag *Bag) String(
	path string,
	def ...string,
) string {
	return get(bag, path, def...)
}

func (bag *Bag) StringMap(
	path string,
	def ...map[string]any,
) map[string]any {
	return get(bag, path, def...)
}

func (bag *Bag) StringMapString(
	path string,
	def ...map[string]string,
) map[string]string {
	return get(bag, path, def...)
}

func (bag *Bag) Slice(
	path string,
	def ...[]any,
) []any {
	return get(bag, path, def...)
}

func (bag *Bag) StringSlice(
	path string,
	def ...[]string,
) []string {
	return get(bag, path, def...)
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
		return append(def, time.Duration(0))[0]
	}
}

func (bag *Bag) Bag(
	path string,
	def ...Bag,
) Bag {
	return get(bag, path, def...)
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
		if srcBag, ok := asBag(value); ok {
			if localBag, ok := asBag((*bag)[key]); ok {
				localBag.Merge(srcBag)
			} else {
				v := Bag{}
				v.Merge(srcBag)
				(*bag)[key] = v
			}
		} else {
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
	var it any

	it = *bag
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}

		if b, ok := asBag(it); ok {
			if it, ok = b[part]; !ok {
				return nil, newErrBagInvalidPath(path)
			}
		} else {
			return nil, newErrBagInvalidPath(path)
		}
	}

	return it, nil
}

func BagNormalization(
	val any,
) any {
	if lValue, ok := val.([]any); ok {
		var result []any
		for _, i := range lValue {
			result = append(result, BagNormalization(i))
		}

		return result
	}

	var mValue map[string]any
	if pValue, ok := val.(Bag); ok {
		mValue = pValue
	} else if mValue, ok = val.(map[string]any); ok {
		// mValue already set
	} else {
		mValue = nil
	}

	if mValue != nil {
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
