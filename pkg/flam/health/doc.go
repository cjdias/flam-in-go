// Package health is the health feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IHealthRegistry] of an
// application, running its health checks for its liveness and readiness
// probes, and the
// [github.com/cjdias/flam-in-go/pkg/flam.IHealthCheckFactory] creating the
// checks described by the configuration at [CheckConfigPath] with the
// creators provided in the [CheckCreatorGroup] container group, such as:
//
//	flam:
//	  health:
//	    timeout: 2s
//	    cache: 1s
//	    checks:
//	      main: {driver: flam.health.checks.driver.database}
//	      cache: {driver: flam.health.checks.driver.redis, connection: main}
//	      servers: {driver: flam.health.checks.driver.kennel, kind: both}
//
// The application may also provide its own checks, made with [NewCheck],
// in the [CheckGroup] container group, such as:
//
//	app.Provide(func() (flam.IHealthCheck, error) {
//		return health.NewCheck("disk", flam.HealthReadiness, checkDisk)
//	}, dig.Group(health.CheckGroup))
//
// The checks are added to the registry when the application boots. A
// probe runs its checks concurrently, each on a goroutine of its own,
// waiting for each up to the timeout at [TimeoutConfigPath], the check
// then failing with an error wrapping [ErrTimeout], while a check that
// panics fails with an error wrapping [ErrPanic]. The result of a check
// may be reused for the time at [CacheConfigPath], by both probes. The
// probe fails when one of its checks fails, and succeeds otherwise, as
// when it has none.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/health/checks], reading the
// [ConfigFieldKind] field shared by the drivers, along with their own,
// while [github.com/cjdias/flam-in-go/pkg/flam/health/endpoints] serves the
// probes as REST endpoints. They import this package for its paths,
// groups and fields, so this package never imports them.
package health
