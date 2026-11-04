package scheduler

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// NewBundle returns the [flam.IBundle] of the scheduler feature,
// providing to the application container, in the [watchdog.ProcessGroup]
// group, the scheduler process, of id [ProcessId], running the jobs of
// the [JobGroup] group, configured at [JobConfigPath] by their id, so
// that the kennel of [github.com/cjdias/flam-in-go/pkg/flam/watchdog]
// runs it, as configured at [watchdog.ProcessConfigPath] by its id. No
// process is provided when the group holds no job.
//
// The process takes from the container the [flam.ICronTriggerFactory]
// creating the triggers of the jobs, so register the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/trigger] too, and the
// [flam.IClock] measuring the runs and their timeouts. It takes, when the
// container holds them, the [flam.ILockerFactory] of
// [github.com/cjdias/flam-in-go/pkg/flam/lock], giving the lockers of the
// singleton jobs, and the [flam.ILogger] it signals the start, the end,
// the failure and the skip of each run to, on the channel at
// [LogChannelConfigPath]. The configuration is read, and the lockers
// taken, once, when the process is created, the provider of the process
// failing with the error of the first invalid job.
//
// The bundle holds no state: the process is owned by the kennel, which
// closes it.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IBundle
var _ flam.IBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(b.provideProcesses, dig.Group(watchdog.ProcessGroup+",flatten")).
		Register()
}

// processArgs are the dependencies of the scheduler process, taken from
// the container. The group tag repeats JobGroup, as a struct tag cannot
// use a constant.
type processArgs struct {
	dig.In

	Config   flam.IConfig
	Clock    flam.IClock
	Triggers flam.ICronTriggerFactory
	Jobs     []flam.IScheduledJob `group:"flam.scheduler.jobs"`
	Lockers  flam.ILockerFactory  `optional:"true"`
	Logger   flam.ILogger         `optional:"true"`
}

// provideProcesses provides the scheduler process, unless there is no
// job, as the processes of the kennel.
func (b *bundleImpl) provideProcesses(
	args processArgs,
) ([]flam.IWatchdogProcess, error) {
	if len(args.Jobs) == 0 {
		return nil, nil
	}
	jobs, err := newJobs(args.Config, args.Lockers, args.Jobs)
	if err != nil {
		return nil, err
	}
	s := newScheduler(schedulerConfig{
		jobs:     jobs,
		triggers: args.Triggers,
		clock:    args.Clock,
		logger:   args.Logger,
		channel:  args.Config.String(LogChannelConfigPath, DefaultLogChannel),
	})
	return []flam.IWatchdogProcess{s}, nil
}
