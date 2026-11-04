package jobs

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// NewBundle returns the [flam.IBundle] of the jobs feature, providing to
// the application container:
//   - a single [flam.IJobQueueFactory], created on first use from the
//     configuration at [QueueConfigPath] and the creators of the
//     [QueueCreatorGroup] group, reading the configuration once;
//   - in the QueueCreatorGroup group, the creator of the queues whose
//     configuration has no driver, or the [Driver] driver, storing their
//     jobs in the [Table] table of their database connection, running
//     them with the handlers of the [HandlerGroup] group, and reading the
//     time from the [flam.IClock] of the container;
//   - the worker of every configured queue, in the
//     [watchdog.ProcessGroup] group, so that the kennel of
//     [github.com/cjdias/flam-in-go/pkg/flam/watchdog] runs them, as
//     configured at [watchdog.ProcessConfigPath] by their id.
//
// Register the bundles of the database feature, of the clock and of the
// watchdog, and the one of the broker feature when the queues publish
// wakeup notifications, along with this one.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the queue factory, and so its queues, stopping their
// workers. As the factory is taken from the container, closing an
// application that never used it creates it first, and fails when it
// cannot be created. As the bundles close in the reverse order of their
// registration, register the database bundle before this one.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IClosableBundle
var _ flam.IClosableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newQueueFactory).
		QueueProvider(newCreator, dig.Group(QueueCreatorGroup)).
		QueueProvider(b.provideProcesses, dig.Group(watchdog.ProcessGroup+",flatten")).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeQueueFactory)
}

// provideProcesses provides the workers of the configured queues, in id
// order, as processes of the kennel, failing with the error of the first
// queue that cannot be created.
func (b *bundleImpl) provideProcesses(
	queues flam.IJobQueueFactory,
) ([]flam.IWatchdogProcess, error) {
	ids := queues.Available()
	processes := make([]flam.IWatchdogProcess, 0, len(ids))
	for _, id := range ids {
		q, err := queues.Get(id)
		if err != nil {
			return nil, err
		}
		processes = append(processes, q.Worker())
	}
	return processes, nil
}

func (b *bundleImpl) closeQueueFactory(
	f flam.IJobQueueFactory,
) error {
	return f.Close()
}
