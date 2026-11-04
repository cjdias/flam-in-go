package jobs

import (
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// WithRunAt returns the [flam.JobOption] keeping the job from running
// before the time.
func WithRunAt(
	at time.Time,
) flam.JobOption {
	return func(options *flam.JobOptions) {
		options.RunAt = at
	}
}

// WithDelay returns the [flam.JobOption] keeping the job from running
// before the delay elapses, unless [WithRunAt] sets a time.
func WithDelay(
	delay time.Duration,
) flam.JobOption {
	return func(options *flam.JobOptions) {
		options.Delay = delay
	}
}

// WithMaxAttempts returns the [flam.JobOption] allowing the job the number
// of attempts, in place of the default of its queue.
func WithMaxAttempts(
	attempts int,
) flam.JobOption {
	return func(options *flam.JobOptions) {
		options.MaxAttempts = attempts
	}
}

// WithKey returns the [flam.JobOption] giving the job the deduplication
// key, so that enqueuing it again, while the job of the key is stored,
// adds no job.
func WithKey(
	key string,
) flam.JobOption {
	return func(options *flam.JobOptions) {
		options.Key = key
	}
}

// WithTx returns the [flam.JobOption] inserting the job with the gorm
// transaction, such as the one given to the callback of a Transaction
// method, so that the job is enqueued only when it commits. The
// transaction must be one of the database of the queue.
func WithTx(
	tx *gorm.DB,
) flam.JobOption {
	return func(options *flam.JobOptions) {
		options.Tx = tx
	}
}
