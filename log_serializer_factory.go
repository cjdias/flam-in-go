package flam

import (
	"io"

	"go.uber.org/dig"
)

type LogSerializerFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (LogSerializer, error)
	Store(id string, serializer LogSerializer) error
	Remove(id string) error
	RemoveAll() error
}

type logSerializerFactoryArgs struct {
	dig.In

	Creators      []LogSerializerCreator `group:"flam.log.serializers.creator"`
	FactoryConfig FactoryConfig
}

func newLogSerializerFactory(
	args logSerializerFactoryArgs,
) (LogSerializerFactory, error) {
	var creators []FactoryResourceCreator[LogSerializer]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("LogSerializer"),
		PathLogSerializers)
}
