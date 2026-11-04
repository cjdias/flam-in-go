package trigger

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.trigger"

// Context fields of the [*flam.Error] values returned by the
// [flam.ICronTriggerFactory] methods and passed to the
// [flam.TriggerErrorHandler] values.
const (
	// ErrorFieldID holds the id of the trigger.
	ErrorFieldID = "id"

	// ErrorFieldDelay holds the delay given to the creation method, with
	// [ErrInvalidDelay].
	ErrorFieldDelay = "delay"

	// ErrorFieldSchedule holds the [flam.CronSchedule] given to
	// [flam.ICronTriggerFactory.NewCron], with [ErrInvalidSchedule].
	ErrorFieldSchedule = "schedule"

	// ErrorFieldPanic holds the value the callback panicked with, with
	// [ErrCallbackPanic].
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack trace of the callback panic, as a
	// string, with [ErrCallbackPanic].
	ErrorFieldStack = "stack"
)
