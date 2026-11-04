package flam

import (
	"context"
)

// IScheduledJob is a job of an application run periodically, at the times
// matched by the cron expression configured for its id, by the scheduler
// of [github.com/cjdias/flam-in-go/pkg/flam/scheduler]. As an
// [IResource], its Id method returns that id, which must not be empty nor
// hold a dot, being a key of the configuration.
//
// Run runs the job once, until it ends or its context is done: the
// context is cancelled once the timeout configured for the job elapses,
// [context.Cause] then returning an error wrapping the timeout sentinel
// error of [github.com/cjdias/flam-in-go/pkg/flam/scheduler], and when the
// scheduler stops. The scheduler never runs a job twice at once, and
// recovers its panics. A job must not close the scheduler running it.
type IScheduledJob interface {
	IResource

	Run(ctx context.Context) error
}
