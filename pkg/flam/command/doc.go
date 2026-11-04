// Package command is the command line feature: the bundle returned from
// [NewBundle] runs, when the application runs, the
// [github.com/cjdias/flam-in-go/pkg/flam.ICommand] named by the first
// argument of the command line, among those the application provides in
// the [CommandGroup] container group, in place of its long running
// processes, such as:
//
//	app migrate up
//	app help
//
// The command line is the arguments of the program, unless set at
// [ArgsConfigPath], such as by the tests. Without argument, or when the
// first argument is a flag, starting with a dash, the command line names
// no command, and the application runs as usual. A named command keeps
// the bundle of [github.com/cjdias/flam-in-go/pkg/flam/watchdog] from
// running the long running processes, by storing false at its
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog.RunConfigPath] when the
// application boots, so that the run of the application ends with the
// command.
//
// The command runs with the arguments following its name and a context
// cancelled on the first interrupt or SIGTERM signal, unless
// [SignalsConfigPath] holds false, a second one ending the program as
// usual, and writes to the io.Writer named [OutputName] in the container,
// or else to the standard output. The built-in [HelpCommand] lists the
// commands with their description, or describes the command it names. A
// name no command has fails the run with [ErrUnknownCommand], whose text
// lists the available commands.
//
// The ready-made commands live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/command/commands], each with a
// bundle providing its command in the CommandGroup group.
package command
