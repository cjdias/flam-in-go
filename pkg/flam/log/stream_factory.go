package log

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// streamFactoryArgs are the dependencies of the stream factory, taken from
// the container. The group tag repeats StreamCreatorGroup, as a struct tag
// cannot use a constant.
type streamFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ILogStreamCreator `group:"flam.log.streams.creator"`
}

// newStreamFactory returns the stream factory creating the streams
// configured at StreamConfigPath with the creators of the
// StreamCreatorGroup group.
func newStreamFactory(
	args streamFactoryArgs,
) (flam.ILogStreamFactory, error) {
	return factory.New(args.Config.Bag(StreamConfigPath), args.Creators...)
}
