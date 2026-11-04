package migration

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.command.commands.migration"

// Name is the name of the migrate command, the first argument of the
// command line selecting it.
const Name = "migrate"

// Actions of the migrate command, its first argument.
const (
	// ActionUp applies every pending migration of the migrators. It is the
	// action of the command without argument.
	ActionUp = "up"

	// ActionDown reverts the last applied migration of the migrators.
	ActionDown = "down"

	// ActionStatus lists the migrations of the migrators, applied or
	// pending.
	ActionStatus = "status"
)

// Context fields of the [*flam.Error] values returned by the migrate
// command.
const (
	// ErrorFieldAction holds the action of the command.
	ErrorFieldAction = "action"

	// ErrorFieldID holds the id of a migrator.
	ErrorFieldID = "id"
)

// description is the description of the migrate command.
const description = "apply (up, the default), revert the last (down) or list (status) the migrations of the migrators, all or those named: migrate [up|down|status] [migrator...]"
