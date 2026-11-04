package endpoints

import (
	"errors"
)

var (
	// ErrInvalidField is the error wrapped by the error of the container
	// when the register cannot be created, as the value at
	// [GroupConfigPath], [LivenessConfigPath] or [ReadinessConfigPath] is
	// not a non-empty string, the path being held by [ErrorFieldField].
	ErrInvalidField = errors.New("invalid health endpoints field")

	// ErrUnhealthy is the error of the envelope answering, with a 503
	// status, a request to the endpoint of a probe that failed.
	ErrUnhealthy = errors.New("health probe failed")
)
