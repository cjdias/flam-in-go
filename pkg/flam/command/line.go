package command

import (
	"os"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newLine returns the command line of the application, read from the
// configuration at [ArgsConfigPath], or else from os.Args. It is a
// provider registered by the bundle returned from [NewBundle].
func newLine(
	config flam.IConfig,
) (*line, error) {
	args := os.Args[1:]
	if config.Has(ArgsConfigPath) {
		var ok bool
		if args, ok = settings.Strings(config.Get(ArgsConfigPath)); !ok {
			return nil, newErrArgs(lineMessage)
		}
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return &line{}, nil
	}
	return &line{args: append([]string(nil), args...)}, nil
}

// line is the command line of the application: the name of the command
// to run, followed by its arguments, or nothing when the application runs
// no command, as no argument is given or the first one is a flag.
type line struct {
	args []string
}

// HasCommand reports whether the command line names a command to run.
func (l *line) HasCommand() bool {
	return len(l.args) > 0
}

// lineMessage is the message of the errors returned by newLine.
const lineMessage = "unable to read command line"
