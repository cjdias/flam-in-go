package health

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// newRegistry returns the registry, empty, waiting for each check as
// configured at TimeoutConfigPath, and reusing its result as configured
// at CacheConfigPath, reading the time from the clock. It fails with
// ErrInvalidField for a value that is not a non-negative duration. It is
// a provider registered by the bundle returned from [NewBundle].
func newRegistry(
	config flam.IConfig,
	clock flam.IClock,
) (flam.IHealthRegistry, error) {
	timeout, ok := settings.Duration(config.Get(TimeoutConfigPath, defaultTimeout))
	if !ok {
		return nil, flam.NewWrappedError(ErrInvalidField, registryNewMessage).
			With(ErrorFieldField, TimeoutConfigPath)
	}
	cache, ok := settings.Duration(config.Get(CacheConfigPath, defaultCache))
	if !ok {
		return nil, flam.NewWrappedError(ErrInvalidField, registryNewMessage).
			With(ErrorFieldField, CacheConfigPath)
	}
	return &registry{
		clock:   clock,
		timeout: timeout,
		cache:   cache,
		checks:  map[string]flam.IHealthCheck{},
		results: map[string]result{},
	}, nil
}

// registry is the [flam.IHealthRegistry] provided by the bundle, while mu
// guards its checks and their cached results.
type registry struct {
	clock   flam.IClock
	timeout time.Duration
	cache   time.Duration

	mu      sync.Mutex
	checks  map[string]flam.IHealthCheck
	results map[string]result
}

// result is the cached report of a check, with the time it ended at.
type result struct {
	report flam.HealthCheckReport
	at     time.Time
}

// compile time check that *registry implements IHealthRegistry
var _ flam.IHealthRegistry = (*registry)(nil)

func (r *registry) Available() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := make([]string, 0, len(r.checks))
	for id := range r.checks {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (r *registry) Has(
	id string,
) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.checks[id]
	return ok
}

// Add fails with ErrNilCheck for a nil check, ErrInvalidID for an empty
// id, ErrInvalidKind for an invalid kind, and ErrDuplicateCheck when the
// id is in use.
func (r *registry) Add(
	check flam.IHealthCheck,
) error {
	if value.IsNil(check) {
		return newErr(ErrNilCheck, registryAddMessage, "", nil)
	}
	id := check.Id()
	if err := validate(registryAddMessage, id, check.Kind()); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.checks[id]; ok {
		return newErr(ErrDuplicateCheck, registryAddMessage, id, nil)
	}
	r.checks[id] = check
	return nil
}

func (r *registry) Liveness(
	ctx context.Context,
) flam.HealthReport {
	return r.probe(ctx, flam.HealthLiveness)
}

func (r *registry) Readiness(
	ctx context.Context,
) flam.HealthReport {
	return r.probe(ctx, flam.HealthReadiness)
}

// Messages of the errors returned by the registry, or reported by it.
const (
	registryNewMessage   = "unable to create health registry"
	registryAddMessage   = "unable to add health check"
	registryCheckMessage = "unable to run health check"
)

// probe runs concurrently the checks of the kind, in id order, and
// returns their report.
func (r *registry) probe(
	ctx context.Context,
	kind flam.HealthKind,
) flam.HealthReport {
	checks := r.checksOf(kind)
	report := flam.HealthReport{
		Status: flam.HealthStatusUp,
		Checks: make([]flam.HealthCheckReport, len(checks)),
	}
	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Go(func() {
			report.Checks[i] = r.result(ctx, check)
		})
	}
	wg.Wait()
	for _, check := range report.Checks {
		if check.Status != flam.HealthStatusUp {
			report.Status = flam.HealthStatusDown
		}
	}
	return report
}

// checksOf returns the checks of the kind, in id order.
func (r *registry) checksOf(
	kind flam.HealthKind,
) []flam.IHealthCheck {
	r.mu.Lock()
	defer r.mu.Unlock()

	var checks []flam.IHealthCheck
	for _, check := range r.checks {
		if check.Kind()&kind != 0 {
			checks = append(checks, check)
		}
	}
	slices.SortFunc(checks, func(a, b flam.IHealthCheck) int {
		return strings.Compare(a.Id(), b.Id())
	})
	return checks
}

// result returns the cached report of the check, while fresh, or else
// runs the check and caches its report, unless the context of the probe
// is done, as its failure would then tell nothing of the check.
func (r *registry) result(
	ctx context.Context,
	check flam.IHealthCheck,
) flam.HealthCheckReport {
	id := check.Id()
	if r.cache > 0 {
		r.mu.Lock()
		cached, ok := r.results[id]
		r.mu.Unlock()
		if ok && r.clock.Since(cached.at) < r.cache {
			return cached.report
		}
	}

	report := r.run(ctx, check)
	if r.cache > 0 && ctx.Err() == nil {
		r.mu.Lock()
		r.results[id] = result{report: report, at: r.clock.Now()}
		r.mu.Unlock()
	}
	return report
}

// run runs the check and returns its report.
func (r *registry) run(
	ctx context.Context,
	check flam.IHealthCheck,
) flam.HealthCheckReport {
	begin := r.clock.Now()
	err := r.wait(ctx, check)
	report := flam.HealthCheckReport{
		Id:       check.Id(),
		Status:   flam.HealthStatusUp,
		Duration: r.clock.Since(begin),
	}
	if err != nil {
		report.Status = flam.HealthStatusDown
		report.Error = err.Error()
	}
	return report
}

// wait runs the check on a goroutine of its own, and waits for it until
// its context is done: the context of the probe, or the timeout of the
// registry, whose cause, an error wrapping ErrTimeout, is then returned.
// A check that does not end with its context keeps running, its result
// being dropped.
func (r *registry) wait(
	ctx context.Context,
	check flam.IHealthCheck,
) error {
	id := check.Id()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	if r.timeout > 0 {
		timer := r.clock.AfterFunc(r.timeout, func() {
			cancel(newErr(ErrTimeout, registryCheckMessage, id, nil))
		})
		defer timer.Stop()
	}

	done := make(chan error, 1)
	go func() {
		done <- call(ctx, id, check)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// call calls the check, returning the error of its panic, if any.
func call(
	ctx context.Context,
	id string,
	check flam.IHealthCheck,
) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = newErrPanic(registryCheckMessage, id, recovered)
		}
	}()
	return check.Check(ctx)
}
