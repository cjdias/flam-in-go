package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"unicode"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// runnerParams are the dependencies of the runner, taken from the
// container.
type runnerParams struct {
	dig.In

	Config   flam.IConfig
	Commands []flam.ICommand `group:"flam.command.commands"`
	Output   io.Writer       `name:"flam.command.output" optional:"true"`
}

// newRunner returns the runner of the commands of [CommandGroup], writing
// to the output of the container, or else to the standard output. It
// fails when a command is nil, has an invalid name, or has the name of
// another one or of [HelpCommand]. It is a provider registered by the
// bundle returned from [NewBundle].
func newRunner(
	params runnerParams,
) (*runner, error) {
	r := &runner{
		config:   params.Config,
		commands: map[string]flam.ICommand{},
		names:    []string{HelpCommand},
		out:      params.Output,
	}
	if r.out == nil {
		r.out = os.Stdout
	}
	for _, c := range params.Commands {
		if c == nil {
			return nil, flam.NewWrappedError(ErrNilCommand, runnerCreateMessage)
		}
		name := c.Name()
		switch {
		case !validName(name):
			return nil, newErrCommand(ErrInvalidName, runnerCreateMessage, name)
		case slices.Contains(r.names, name):
			return nil, newErrCommand(ErrDuplicateCommand, runnerCreateMessage, name)
		}
		r.commands[name] = c
		r.names = append(r.names, name)
	}
	slices.Sort(r.names)
	return r, nil
}

// runner runs the command named by the command line, among the commands
// of [CommandGroup] and the built-in [HelpCommand].
type runner struct {
	config   flam.IConfig
	commands map[string]flam.ICommand
	names    []string
	out      io.Writer
}

// Run runs the command named by the first argument with the others, with
// a context cancelled on an interrupt or a SIGTERM signal, unless
// [SignalsConfigPath] holds false. It returns the error of the command,
// wrapped, or fails with ErrUnknownCommand when no command has the name.
func (r *runner) Run(
	args []string,
) error {
	name := args[0]
	if name == HelpCommand {
		return r.help(args[1:])
	}
	c, ok := r.commands[name]
	if !ok {
		return newErrUnknown(runnerRunMessage, name, r.names)
	}

	ctx, stop := signalContext(r.config.Bool(SignalsConfigPath, defaultSignals))
	defer stop()
	if err := c.Run(ctx, args[1:], r.out); err != nil {
		return newErrCommand(err, runnerRunMessage, name)
	}
	return nil
}

// help writes the usage of the command line and the commands with their
// description, or the description of the command named by the first
// argument, failing with ErrUnknownCommand when no command has the name,
// and with the error of the output.
func (r *runner) help(
	args []string,
) error {
	if len(args) > 0 && args[0] != HelpCommand && r.commands[args[0]] == nil {
		return newErrUnknown(runnerRunMessage, args[0], r.names)
	}

	var b strings.Builder
	if len(args) > 0 {
		fmt.Fprintf(&b, "Usage: %s [arguments]\n\n%s\n", args[0], r.description(args[0]))
	} else {
		width := 0
		for _, name := range r.names {
			width = max(width, len(name))
		}
		b.WriteString("Usage: <command> [arguments]\n\nCommands:\n")
		for _, name := range r.names {
			fmt.Fprintf(&b, "  %-*s  %s\n", width, name, r.description(name))
		}
	}
	if _, err := io.WriteString(r.out, b.String()); err != nil {
		return newErrCommand(err, runnerRunMessage, HelpCommand)
	}
	return nil
}

// description returns the description of the command of the name.
func (r *runner) description(
	name string,
) string {
	if name == HelpCommand {
		return helpDescription
	}
	return r.commands[name].Description()
}

// validName reports whether the name is a valid command name: non-empty,
// without white space, and not starting with a dash.
func validName(
	name string,
) bool {
	return name != "" &&
		!strings.HasPrefix(name, "-") &&
		!strings.ContainsFunc(name, unicode.IsSpace)
}

// signalContext returns a context and the function releasing it, the
// context being cancelled on the first interrupt or SIGTERM signal when
// signals is true, but for the signals the program started ignoring, as
// listening to a signal stops ignoring it. A second signal ends the
// program as usual.
func signalContext(
	signals bool,
) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	listened := listenedSignals()
	if !signals || len(listened) == 0 {
		return ctx, cancel
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, listened...)
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-sigs:
			signal.Stop(sigs)
			cancel()
		case <-done:
		}
	}()
	return ctx, func() {
		signal.Stop(sigs)
		close(done)
		<-stopped
		cancel()
	}
}

// listenedSignals returns the signals cancelling the context of a
// command, the interrupt and SIGTERM, but those the program started
// ignoring.
func listenedSignals() []os.Signal {
	var listened []os.Signal
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		if !signal.Ignored(sig) {
			listened = append(listened, sig)
		}
	}
	return listened
}

// Messages of the errors returned by the runner.
const (
	runnerCreateMessage = "unable to create command runner"
	runnerRunMessage    = "unable to run command"
)
