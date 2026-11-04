package command

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.command"

// Configuration paths, container group and container value names of the
// command line.
const (
	// CommandGroup is the container group the application provides its
	// [flam.ICommand] values in, run by the bundle returned from
	// [NewBundle].
	CommandGroup = "flam.command.commands"

	// ArgsConfigPath is the configuration path of the arguments of the
	// command line, a list of strings, the first one naming the command to
	// run. When unset, they are the arguments of the program, os.Args
	// without the program name. An empty list runs no command.
	ArgsConfigPath = "flam.command.args"

	// SignalsConfigPath is the configuration path of a bool telling
	// whether the context given to the command is cancelled on an
	// interrupt or a SIGTERM signal, a second signal ending the program as
	// usual. It defaults to true.
	SignalsConfigPath = "flam.command.signals"

	// OutputName is the name of the optional io.Writer value of the
	// container the commands write their output to, the standard output
	// when not provided.
	OutputName = "flam.command.output"
)

// HelpCommand is the name of the built-in command listing the commands
// with their description, or describing the command named by its
// argument.
const HelpCommand = "help"

// Context fields of the [*flam.Error] values returned by the command
// package.
const (
	// ErrorFieldCommand holds the name of a command.
	ErrorFieldCommand = "command"

	// ErrorFieldCommands holds, sorted, the names of the available
	// commands.
	ErrorFieldCommands = "commands"

	// ErrorFieldField holds the configuration path of an invalid value.
	ErrorFieldField = "field"
)

// Defaults of the command bundle behaviour.
const (
	defaultSignals = true

	// helpDescription is the description of the help command.
	helpDescription = "list the commands, or describe the named one"
)
