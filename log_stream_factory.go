package flam

import (
	"io"

	"go.uber.org/dig"
)

type LogStreamFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (LogStream, error)
	Store(id string, stream LogStream) error
	Remove(id string) error
	RemoveAll() error
}

type logStreamFactory struct {
	factory *factory[LogStream]
	logger  *logger
}

type logSteamFactoryArgs struct {
	dig.In

	Creators      []LogStreamCreator `group:"flam.log.streams.creator"`
	FactoryConfig FactoryConfig
	Logger        *logger
}

func newLogStreamFactory(
	args logSteamFactoryArgs,
) (LogStreamFactory, error) {
	var creators []FactoryResourceCreator[LogStream]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	f, _ := NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("LogStream"),
		PathLogStreams)

	return &logStreamFactory{
		factory: f.(*factory[LogStream]),
		logger:  args.Logger}, nil
}

func (factory logStreamFactory) Close() error {
	return factory.factory.Close()
}

func (factory logStreamFactory) Available() []string {
	return factory.factory.Available()
}

func (factory logStreamFactory) Stored() []string {
	return factory.factory.Stored()
}

func (factory logStreamFactory) Has(
	id string,
) bool {
	return factory.factory.Has(id)
}

func (factory logStreamFactory) Get(
	id string,
) (LogStream, error) {
	log, e := factory.factory.Get(id)
	if e == nil {
		factory.factory.locker.Lock()
		defer factory.factory.locker.Unlock()

		factory.logger.streams = factory.factory.entries
	}

	return log, e
}

func (factory logStreamFactory) Store(
	id string,
	stream LogStream,
) error {
	e := factory.factory.Store(id, stream)
	if e == nil {
		factory.factory.locker.Lock()
		defer factory.factory.locker.Unlock()

		factory.logger.streams = factory.factory.entries
	}

	return e
}

func (factory logStreamFactory) Remove(
	id string,
) error {
	e := factory.factory.Remove(id)
	if e == nil {
		factory.factory.locker.Lock()
		defer factory.factory.locker.Unlock()

		factory.logger.streams = factory.factory.entries
	}

	return e
}

func (factory logStreamFactory) RemoveAll() error {
	e := factory.factory.RemoveAll()
	if e == nil {
		factory.factory.locker.Lock()
		defer factory.factory.locker.Unlock()

		factory.logger.streams = factory.factory.entries
	}

	return e
}
