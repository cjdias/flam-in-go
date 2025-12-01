package flam

type stringLogSerializerCreator struct{}

var _ LogSerializerCreator = (*stringLogSerializerCreator)(nil)

func newStringLogSerializerCreator() LogSerializerCreator {
	return &stringLogSerializerCreator{}
}

func (stringLogSerializerCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == LogSerializerDriverString
}

func (stringLogSerializerCreator) Create(
	_ Bag,
) (LogSerializer, error) {
	return newStringLogSerializer(), nil
}
