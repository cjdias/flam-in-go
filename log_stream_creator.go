package flam

type LogStreamCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (LogStream, error)
}

type logStreamCreator struct {
	logSerializerFactory LogSerializerFactory
}

func (creator logStreamCreator) getChannels(
	list []any,
) []string {
	var result []string
	for _, channel := range list {
		if typedChannel, ok := channel.(string); ok {
			result = append(result, typedChannel)
		}
	}

	return result
}
