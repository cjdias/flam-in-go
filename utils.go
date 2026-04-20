package flam

import "reflect"

func mergeContext(ctx ...Bag) Bag {
	context := Bag{}
	for _, c := range ctx {
		context.Merge(c)
	}
	return context
}

func asBag(
	value any,
) (Bag, bool) {
	switch v := value.(type) {
	case Bag:
		return v, true
	case *Bag:
		return *v, true
	default:
		return nil, false
	}
}

func get[T any](
	bag *Bag,
	path string,
	def ...T,
) T {
	if val, ok := bag.Get(path).(T); ok {
		return val
	}
	return append(def, *new(T))[0]
}

func isNil(resource any) bool {
	if resource == nil {
		return true
	}
	return reflect.ValueOf(resource).IsNil()
}
