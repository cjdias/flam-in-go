// Package trigger holds the implementation of the
// [github.com/cjdias/flam-in-go/pkg/flam.ITriggerFactory] and
// [github.com/cjdias/flam-in-go/pkg/flam.ICronTriggerFactory] contracts,
// provided to the application container by the bundle returned from
// [NewBundle], creating pulse and recurring
// [github.com/cjdias/flam-in-go/pkg/flam.ITrigger] timers, and
// [github.com/cjdias/flam-in-go/pkg/flam.ICronTrigger] timers firing at
// the times matched by a cron expression, parsed with
// [github.com/robfig/cron/v3], but waiting through the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, so
// that a fake clock controls when they fire.
package trigger
