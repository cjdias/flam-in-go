package flam

import (
	"errors"
	"fmt"
)

var (
	ErrNilReference                      = errors.New("nil reference")
	ErrBagInvalidPath                    = errors.New("invalid bag path")
	ErrUnknownResource                   = errors.New("unknown resource")
	ErrInvalidResourceConfig             = errors.New("invalid resource config")
	ErrUnacceptedResourceConfig          = errors.New("unaccepted resource config")
	ErrDuplicateResource                 = errors.New("duplicate resource")
	ErrSubscriptionNotFound              = errors.New("subscription not found")
	ErrDuplicateSubscription             = errors.New("duplicate subscription")
	ErrDuplicateProvider                 = errors.New("duplicate provider")
	ErrRestConfigSourceConfigNotFound    = errors.New("config rest source config source data not found")
	ErrInvalidRestConfigSourceConfig     = errors.New("invalid config rest source config source data")
	ErrRestConfigSourceTimestampNotFound = errors.New("config rest source config source timestamp not found")
	ErrInvalidRestConfigSourceTimestamp  = errors.New("invalid config rest source config source timestamp")
	ErrDuplicateConfigObserver           = errors.New("duplicate config observer")
	ErrUnknownDatabaseLogType            = errors.New("unknown database log type")
	ErrUnknownDatabaseLogLevel           = errors.New("unknown database log level")
	ErrMissingCacheObject                = errors.New("missing cache object")
	ErrLanguageNotFound                  = errors.New("translator language not found")
	ErrProcessNotFound                   = errors.New("watchdog process not found")
	ErrProcessIsRunning                  = errors.New("watchdog process is currently running")
	ErrProcessRunningError               = errors.New("watchdog process running error")
)

func newErrNilReference(
	arg string,
) error {
	return NewErrorFrom(ErrNilReference, arg)
}

func newErrBagInvalidPath(
	path string,
) error {
	return NewErrorFrom(ErrBagInvalidPath, path)
}

func newErrUnknownResource(
	resource string,
	id string,
) error {
	return NewErrorFrom(ErrUnknownResource, fmt.Sprintf("%s[%s]", resource, id))
}

func newErrInvalidResourceConfig(
	resource string,
	field string,
	config Bag,
) error {
	return NewErrorFrom(ErrInvalidResourceConfig, fmt.Sprintf("%s[%s] => %v", resource, field, config))
}

func newErrUnacceptedResourceConfig(
	resource string,
	config Bag,
) error {
	return NewErrorFrom(ErrUnacceptedResourceConfig, fmt.Sprintf("%s => %v", resource, config))
}

func newErrDuplicateResource(
	id string,
) error {
	return NewErrorFrom(ErrDuplicateResource, id)
}

func newErrSubscriptionNotFound[I PubSubID, C PubSubChannel](
	id I,
	channel C,
) error {
	return NewErrorFrom(ErrSubscriptionNotFound, fmt.Sprintf("%v[%v]", id, channel))
}

func newErrDuplicateSubscription[I PubSubID, C PubSubChannel](
	id I,
	channel C,
) error {
	return NewErrorFrom(ErrDuplicateSubscription, fmt.Sprintf("%v[%v]", id, channel))
}

func newErrDuplicateProvider(
	id string,
) error {
	return NewErrorFrom(ErrDuplicateProvider, id)
}

func newErrRestConfigSourceConfigNotFound(
	path string,
	config Bag,
) error {
	return NewErrorFrom(ErrRestConfigSourceConfigNotFound, fmt.Sprintf("%s => %v", path, config))
}

func newErrInvalidRestConfigSourceConfig(
	path string,
	value any,
) error {
	return NewErrorFrom(ErrInvalidRestConfigSourceConfig, fmt.Sprintf("%s => %v", path, value))
}

func newErrRestConfigSourceTimestampNotFound(
	path string,
	config Bag,
) error {
	return NewErrorFrom(ErrRestConfigSourceTimestampNotFound, fmt.Sprintf("%s => %v", path, config))
}

func newErrInvalidRestConfigSourceTimestamp(
	path string,
	value any,
) error {
	return NewErrorFrom(ErrInvalidRestConfigSourceTimestamp, fmt.Sprintf("%s => %v", path, value))
}

func newErrDuplicateConfigObserver(
	path string,
	id string,
) error {
	return NewErrorFrom(ErrDuplicateConfigObserver, fmt.Sprintf("%s => %s", path, id))
}

func newErrUnknownDatabaseLogType(
	logger string,
) error {
	return NewErrorFrom(ErrUnknownDatabaseLogType, logger)
}

func newErrUnknownDatabaseLogLevel(
	level string,
) error {
	return NewErrorFrom(ErrUnknownDatabaseLogLevel, level)
}

func newErrLanguageNotFound(
	language string,
) error {
	return NewErrorFrom(ErrLanguageNotFound, language)
}

func newErrProcessNotFound(
	id string,
) error {
	return NewErrorFrom(ErrProcessNotFound, id)
}

func newErrProcessIsRunning(
	id string,
) error {
	return NewErrorFrom(ErrProcessIsRunning, id)
}

func newErrProcessRunningError(
	result any,
) error {
	return NewErrorFrom(ErrProcessRunningError, fmt.Sprintf("%v", result))
}
