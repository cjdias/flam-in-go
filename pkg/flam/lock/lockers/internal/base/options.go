package base

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
)

// Options are the options of a locker, read from the fields shared by the
// locker drivers.
type Options struct {
	// RetryInterval is the time Acquire waits before trying again.
	RetryInterval time.Duration

	// RetryJitter is the maximum of the random time added to each wait.
	RetryJitter time.Duration

	// AutoRefresh tells whether the locker refreshes its locks.
	AutoRefresh bool
}

// ReadOptions returns the options of the fields shared by the locker
// drivers, or of their defaults, along with the first invalid field, or
// "" when none is.
func ReadOptions(
	config flam.IConfig,
	bag flam.IBag,
) (Options, string) {
	interval, ok := settings.Duration(settings.Field(config, bag, lock.ConfigFieldRetryInterval, lock.DefaultRetryIntervalConfigPath, lock.DefaultRetryInterval))
	if !ok || interval == 0 {
		return Options{}, lock.ConfigFieldRetryInterval
	}
	jitter, ok := settings.Duration(settings.Field(config, bag, lock.ConfigFieldRetryJitter, lock.DefaultRetryJitterConfigPath, lock.DefaultRetryJitter))
	if !ok {
		return Options{}, lock.ConfigFieldRetryJitter
	}
	autoRefresh, ok := settings.Field(config, bag, lock.ConfigFieldAutoRefresh, lock.DefaultAutoRefreshConfigPath, lock.DefaultAutoRefresh).(bool)
	if !ok {
		return Options{}, lock.ConfigFieldAutoRefresh
	}
	return Options{RetryInterval: interval, RetryJitter: jitter, AutoRefresh: autoRefresh}, ""
}
