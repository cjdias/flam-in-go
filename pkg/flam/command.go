package flam

import (
	"context"
	"io"
)

// ICommand is a command of the command line of an application, run, in
// place of its long running processes, when the application runs with
// the name of the command as first argument, as done by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/command].
type ICommand interface {
	// Name returns the name of the command, the first argument selecting
	// it, unique among the commands of an application. It must be
	// non-empty, hold no white space and not start with a dash, which
	// starts a flag.
	Name() string

	// Description returns the one line description of the command, as
	// listed by the help command.
	Description() string

	// Run runs the command with the arguments following its name, writing
	// its output to out. The context is cancelled on an interrupt or a
	// SIGTERM signal, so that a long command may stop early.
	Run(ctx context.Context, args []string, out io.Writer) error
}
