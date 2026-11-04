package flam

import (
	"context"
	"encoding/json"
	"time"
)

// HealthKind tells which probes of an [IHealthRegistry] a health check
// counts for: the liveness probe, telling whether the application works
// at all, and should be restarted otherwise, the readiness probe, telling
// whether it can serve, such as whether its database answers, or both.
type HealthKind int

// Kinds of the health checks, combined with |.
const (
	// HealthLiveness is the kind of the checks counting for the liveness
	// probe.
	HealthLiveness HealthKind = 1 << iota

	// HealthReadiness is the kind of the checks counting for the
	// readiness probe.
	HealthReadiness

	// HealthBoth is the kind of the checks counting for both probes.
	HealthBoth = HealthLiveness | HealthReadiness
)

// HealthStatus is the status of a health check, or of a probe of an
// [IHealthRegistry].
type HealthStatus string

// Statuses of the health checks and of the probes.
const (
	// HealthStatusUp is the status of a check that succeeded, and of a
	// probe whose checks all succeeded.
	HealthStatusUp HealthStatus = "up"

	// HealthStatusDown is the status of a check that failed, and of a
	// probe of which a check failed.
	HealthStatusDown HealthStatus = "down"
)

// IHealthCheck checks the health of a part of an application, such as a
// database connection, for the probes of an [IHealthRegistry] of its
// kind. It is held by an [IHealthCheckFactory], or provided by the
// application, and may be called from several goroutines.
type IHealthCheck interface {
	IResource

	// Kind returns the probes the check counts for: [HealthLiveness],
	// [HealthReadiness] or [HealthBoth].
	Kind() HealthKind

	// Check returns nil when the part it checks is healthy, or the error
	// telling why it is not. It should end once its context is done, as
	// the registry stops waiting for it then.
	Check(ctx context.Context) error
}

// IHealthPinger is implemented by the resources able to tell cheaply
// whether their backend answers, such as an [IBroker] whose driver
// implements it, for the health checks of the application, which ping
// them rather than doing an actual work with them. Ping returns nil when
// the backend answers, or the error telling why it does not, ending once
// its context is done.
type IHealthPinger interface {
	Ping(ctx context.Context) error
}

// IHealthCheckCreator creates the health checks of a driver, for the
// [IHealthCheckFactory].
type IHealthCheckCreator = IResourceCreator[IHealthCheck]

// IHealthCheckFactory creates and holds the health checks, one per id.
type IHealthCheckFactory = IFactory[IHealthCheck]

// HealthCheckReport is the result of a health check run by a probe of an
// [IHealthRegistry].
type HealthCheckReport struct {
	// Id is the id of the check.
	Id string

	// Status is [HealthStatusUp] when the check succeeded, and
	// [HealthStatusDown] otherwise.
	Status HealthStatus

	// Error is the text of the error of the check, empty when it
	// succeeded.
	Error string

	// Duration is the time the check took, up to the end of the wait for
	// it, such as its timeout.
	Duration time.Duration
}

// MarshalJSON returns the JSON object of the report: its "id", its
// "status", its "error", left out when empty, and its "duration", written
// as text, such as "1.5ms".
func (r HealthCheckReport) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Id       string       `json:"id"`
		Status   HealthStatus `json:"status"`
		Error    string       `json:"error,omitempty"`
		Duration string       `json:"duration"`
	}{
		Id:       r.Id,
		Status:   r.Status,
		Error:    r.Error,
		Duration: r.Duration.String(),
	})
}

// HealthReport is the result of a probe of an [IHealthRegistry]: the
// reports of its checks, and its status, [HealthStatusDown] when one of
// them failed, and [HealthStatusUp] otherwise, as when it has no check.
type HealthReport struct {
	// Status is the status of the probe.
	Status HealthStatus `json:"status"`

	// Checks are the reports of the checks of the probe, in id order.
	Checks []HealthCheckReport `json:"checks"`
}

// Healthy reports whether the status of the report is [HealthStatusUp].
func (r HealthReport) Healthy() bool {
	return r.Status == HealthStatusUp
}

// IHealthRegistry holds the health checks of an application, and runs
// them for its probes: Liveness runs those of the [HealthLiveness] kind,
// and Readiness those of the [HealthReadiness] kind, a check of the
// [HealthBoth] kind counting for both. A probe runs its checks
// concurrently, each within a timeout, and reports their results, a
// failed check making the probe fail. It may be used from several
// goroutines.
type IHealthRegistry interface {
	// Available returns the ids of the checks, sorted.
	Available() []string

	// Has reports whether the registry holds a check of the id.
	Has(id string) bool

	// Add adds the check, failing when it is nil, its id is empty or in
	// use, or its kind is not one of [HealthLiveness], [HealthReadiness]
	// and [HealthBoth].
	Add(check IHealthCheck) error

	// Liveness runs the checks of the liveness probe, and returns their
	// report.
	Liveness(ctx context.Context) HealthReport

	// Readiness runs the checks of the readiness probe, and returns their
	// report.
	Readiness(ctx context.Context) HealthReport
}
