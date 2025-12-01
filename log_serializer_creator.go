package flam

type LogSerializerCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (LogSerializer, error)
}
