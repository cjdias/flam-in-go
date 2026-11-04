package lock

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.lock"

// Configuration path and container group of the lockers.
const (
	// LockerConfigPath is the configuration path holding one locker
	// configuration per locker id, as read by the locker factory.
	LockerConfigPath = "flam.lock.lockers"

	// LockerCreatorGroup is the container group the locker driver bundles
	// provide their [flam.ILockerCreator] in, collected by the locker
	// factory.
	LockerCreatorGroup = "flam.lock.lockers.creator"
)

// Fields of a locker configuration shared by the locker drivers.
const (
	// ConfigFieldRetryInterval is the field holding the time Acquire waits
	// before trying again to acquire a lock another holds, a positive
	// duration such as "100ms".
	ConfigFieldRetryInterval = "retry_interval"

	// ConfigFieldRetryJitter is the field holding the maximum of the
	// random time added to each wait of Acquire, so that the lockers
	// waiting for a key do not try again all at once, a duration such as
	// "50ms", 0 for none.
	ConfigFieldRetryJitter = "retry_jitter"

	// ConfigFieldAutoRefresh is the field holding whether the locker
	// refreshes the locks it holds on its own, a bool: every third of
	// their ttl, with that ttl, until released or lost. A refresh failing
	// on an error of the backend is tried again a third of the ttl later,
	// the lock being lost once its ttl elapsed since its last refresh.
	ConfigFieldAutoRefresh = "auto_refresh"
)

// Configuration paths of the values taken by the shared fields missing
// from a locker configuration.
const (
	// DefaultRetryIntervalConfigPath is the path of the default of
	// [ConfigFieldRetryInterval], unset unless configured, then taking
	// [DefaultRetryInterval].
	DefaultRetryIntervalConfigPath = "flam.lock.defaults.lockers.retry_interval"

	// DefaultRetryJitterConfigPath is the path of the default of
	// [ConfigFieldRetryJitter], unset unless configured, then taking
	// [DefaultRetryJitter].
	DefaultRetryJitterConfigPath = "flam.lock.defaults.lockers.retry_jitter"

	// DefaultAutoRefreshConfigPath is the path of the default of
	// [ConfigFieldAutoRefresh], unset unless configured, then taking
	// [DefaultAutoRefresh].
	DefaultAutoRefreshConfigPath = "flam.lock.defaults.lockers.auto_refresh"
)

// Built-in defaults of the shared fields missing from a locker
// configuration and from the configuration at their default paths.
const (
	// DefaultRetryInterval is the built-in default of
	// [ConfigFieldRetryInterval].
	DefaultRetryInterval = 100 * time.Millisecond

	// DefaultRetryJitter is the built-in default of
	// [ConfigFieldRetryJitter].
	DefaultRetryJitter = 50 * time.Millisecond

	// DefaultAutoRefresh is the built-in default of
	// [ConfigFieldAutoRefresh].
	DefaultAutoRefresh = true
)

// Context fields of the [*flam.Error] values returned by the lockers and
// the locks of every driver.
const (
	// ErrorFieldID holds the id of the locker.
	ErrorFieldID = "id"

	// ErrorFieldKey holds the key of the lock.
	ErrorFieldKey = "key"
)
