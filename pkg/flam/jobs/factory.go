package jobs

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// queueFactoryArgs are the dependencies of the queue factory, taken from
// the container. The group tag repeats QueueCreatorGroup, as a struct tag
// cannot use a constant.
type queueFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IJobQueueCreator `group:"flam.jobs.queues.creator"`
}

// newQueueFactory returns the queue factory creating the queues
// configured at QueueConfigPath with the creators of the
// QueueCreatorGroup group.
func newQueueFactory(
	args queueFactoryArgs,
) (flam.IJobQueueFactory, error) {
	return factory.New(args.Config.Bag(QueueConfigPath), args.Creators...)
}
