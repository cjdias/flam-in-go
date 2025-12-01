package flam

type jsonLogSerializerCreator struct{}

var _ LogSerializerCreator = (*jsonLogSerializerCreator)(nil)

func newJsonLogSerializerCreator() LogSerializerCreator {
	return &jsonLogSerializerCreator{}
}

func (jsonLogSerializerCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == LogSerializerDriverJson
}

func (jsonLogSerializerCreator) Create(
	_ Bag,
) (LogSerializer, error) {
	return newJsonLogSerializer(), nil
}
