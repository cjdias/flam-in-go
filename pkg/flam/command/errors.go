package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidArgs is the error wrapped by the error returned when the
	// application boots, or runs, when the value at [ArgsConfigPath] is
	// not a list of strings.
	ErrInvalidArgs = errors.New("invalid command line arguments")

	// ErrNilCommand is the error wrapped by the error returned when a
	// command of [CommandGroup] is nil.
	ErrNilCommand = errors.New("nil command")

	// ErrInvalidName is the error wrapped by the error returned when the
	// name of a command of [CommandGroup] is empty, holds white space or
	// starts with a dash.
	ErrInvalidName = errors.New("invalid command name")

	// ErrDuplicateCommand is the error wrapped by the error returned when
	// two commands of [CommandGroup], or one of them and the built-in
	// [HelpCommand], have the same name.
	ErrDuplicateCommand = errors.New("duplicate command name")

	// ErrUnknownCommand is the error wrapped by the error returned when no
	// command has the name given on the command line. Its text lists the
	// available commands.
	ErrUnknownCommand = errors.New("unknown command")
)

// newErrArgs returns the error wrapping ErrInvalidArgs with the message
// and the ErrorFieldField context field.
func newErrArgs(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidArgs, message).
		With(ErrorFieldField, ArgsConfigPath)
}

// newErrCommand returns the error wrapping the sentinel with the message
// and the ErrorFieldCommand context field.
func newErrCommand(
	sentinel error,
	message string,
	name string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldCommand, name)
}

// newErrUnknown returns the error wrapping ErrUnknownCommand, with the
// name and the available commands in its text, with the message and the
// ErrorFieldCommand and ErrorFieldCommands context fields.
func newErrUnknown(
	message string,
	name string,
	available []string,
) *flam.Error {
	cause := fmt.Errorf("%w %q, available: %s", ErrUnknownCommand, name, strings.Join(available, ", "))
	return newErrCommand(cause, message, name).
		With(ErrorFieldCommands, available)
}
