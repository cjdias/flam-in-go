package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/command"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// cmd is a command recording its runs and running fn, when set.
type cmd struct {
	name, description string
	fn                func(ctx context.Context, args []string, out io.Writer) error

	mu   sync.Mutex
	runs [][]string
	outs []io.Writer
}

// newCmd returns the command of the name, writing its arguments.
func newCmd(name string) *cmd {
	return &cmd{name: name, description: "the " + name + " command"}
}

func (c *cmd) Name() string        { return c.name }
func (c *cmd) Description() string { return c.description }

func (c *cmd) Run(ctx context.Context, args []string, out io.Writer) error {
	c.mu.Lock()
	c.runs = append(c.runs, args)
	c.outs = append(c.outs, out)
	c.mu.Unlock()
	if c.fn != nil {
		return c.fn(ctx, args, out)
	}
	_, err := io.WriteString(out, c.name+" ran\n")
	return err
}

// calls returns the arguments of the runs of the command.
func (c *cmd) calls() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]string(nil), c.runs...)
}

// process is a watchdog process counting its runs.
type process struct {
	mu   sync.Mutex
	runs int
}

func (p *process) Id() string      { return "api" }
func (p *process) IsRunning() bool { return false }
func (p *process) Close() error    { return nil }

func (p *process) Run(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runs++
	return nil
}

// count returns the number of runs of the process.
func (p *process) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runs
}

// failingWriter is a writer failing every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errBoom }

// values returns the bag of the values, by path.
func values(t *testing.T, vals map[string]any) container.Bag {
	t.Helper()
	bag := container.Bag{}
	for path, value := range vals {
		testutil.NoError(t, bag.Set(path, value))
	}
	return bag
}

// provide provides the commands in the command group of the app, and the
// output, unless nil.
func provide(t *testing.T, a flam.IApp, out io.Writer, commands ...flam.ICommand) {
	t.Helper()
	for _, c := range commands {
		testutil.NoError(t, a.Provide(func() flam.ICommand { return c }, dig.Group(command.CommandGroup)))
	}
	if out != nil {
		testutil.NoError(t, a.Provide(func() io.Writer { return out }, dig.Name(command.OutputName)))
	}
}

// newApp returns a testutil.App with the config and command bundles, the
// values as configuration defaults, the commands and the output.
func newApp(t *testing.T, vals map[string]any, out io.Writer, commands ...flam.ICommand) (*testutil.App, flam.IBundle) {
	t.Helper()
	a := testutil.NewApp()
	b := command.NewBundle()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(b))
	provide(t, a, out, commands...)
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(values(t, vals)) }))
	return a, b
}

// boot boots the command bundle of the app.
func boot(a flam.IApp, b flam.IBundle) error {
	return b.(flam.IBootableBundle).Boot(a)
}

// run runs the command bundle of the app.
func run(a flam.IApp, b flam.IBundle) error {
	return b.(flam.IRunnableBundle).Run(a)
}

// args returns the configuration values setting the command line.
func args(list ...string) map[string]any {
	return map[string]any{command.ArgsConfigPath: list}
}

// watchdogRun returns the value stored at watchdog.RunConfigPath in the
// configuration of the app, or "unset".
func watchdogRun(t *testing.T, a flam.IApp) any {
	t.Helper()
	var v any
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) { v = c.Get(watchdog.RunConfigPath, "unset") }))
	return v
}

// flamError fails the test unless err holds a *flam.Error wrapping
// sentinel, the first of its chain having the text and the context.
func flamError(t *testing.T, err, sentinel error, text string, ctx map[string]any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(err, &e) {
		t.Fatalf("error = %v, want a *flam.Error wrapping %v", err, sentinel)
	}
	testutil.Equal(t, "Error()", e.Error(), text)
	testutil.Context(t, e, ctx)
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "CommandGroup", command.CommandGroup, "flam.command.commands")
	testutil.Equal(t, "ArgsConfigPath", command.ArgsConfigPath, "flam.command.args")
	testutil.Equal(t, "SignalsConfigPath", command.SignalsConfigPath, "flam.command.signals")
	testutil.Equal(t, "OutputName", command.OutputName, "flam.command.output")
	testutil.Equal(t, "HelpCommand", command.HelpCommand, "help")
	testutil.Equal(t, "ErrorFieldCommand", command.ErrorFieldCommand, "command")
	testutil.Equal(t, "ErrorFieldCommands", command.ErrorFieldCommands, "commands")
	testutil.Equal(t, "ErrorFieldField", command.ErrorFieldField, "field")
	for name, err := range map[string]error{
		"ErrInvalidArgs":      command.ErrInvalidArgs,
		"ErrNilCommand":       command.ErrNilCommand,
		"ErrInvalidName":      command.ErrInvalidName,
		"ErrDuplicateCommand": command.ErrDuplicateCommand,
		"ErrUnknownCommand":   command.ErrUnknownCommand,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the command line and the runner", func(t *testing.T) {
		b := command.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.command")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.command", "provider/0", "provider/0"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		a := testutil.NewApp()
		a.Fail = func(kind string) error {
			if kind == "provider" {
				return errBoom
			}
			return nil
		}
		if err := a.RegisterBundle(command.NewBundle()); !errors.Is(err, errBoom) {
			t.Fatalf("RegisterBundle() = %v, want errBoom", err)
		}
	})
}

func Test_Line(t *testing.T) {
	t.Run("should run nothing without argument", func(t *testing.T) {
		hello := newCmd("hello")
		out := &bytes.Buffer{}
		a, b := newApp(t, args(), out, hello)
		// a failing command provider shows the commands are not taken
		testutil.NoError(t, a.Provide(func() (flam.ICommand, error) { return nil, errBoom }, dig.Group(command.CommandGroup)))
		testutil.NoError(t, boot(a, b))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "runs", hello.calls(), [][]string(nil))
		testutil.Equal(t, "output", out.String(), "")
		testutil.Equal(t, "watchdog run", watchdogRun(t, a), "unset")
	})

	t.Run("should run nothing when the first argument is a flag", func(t *testing.T) {
		hello := newCmd("hello")
		a, b := newApp(t, args("-v", "hello"), nil, hello)
		testutil.NoError(t, boot(a, b))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "runs", hello.calls(), [][]string(nil))
		testutil.Equal(t, "watchdog run", watchdogRun(t, a), "unset")
	})

	t.Run("should take the arguments of a list of values", func(t *testing.T) {
		hello := newCmd("hello")
		a, b := newApp(t, map[string]any{command.ArgsConfigPath: []any{"hello", "a"}}, &bytes.Buffer{}, hello)
		testutil.NoError(t, boot(a, b))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "runs", hello.calls(), [][]string{{"a"}})
	})

	t.Run("should take the arguments of the program by default", func(t *testing.T) {
		saved := os.Args
		defer func() { os.Args = saved }()

		os.Args = []string{"prog", "hello", "a", "b"}
		hello := newCmd("hello")
		a, b := newApp(t, nil, &bytes.Buffer{}, hello)
		testutil.NoError(t, boot(a, b))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "runs", hello.calls(), [][]string{{"a", "b"}})
		testutil.Equal(t, "watchdog run", watchdogRun(t, a), false)

		os.Args = []string{"prog"}
		hello = newCmd("hello")
		a, b = newApp(t, nil, &bytes.Buffer{}, hello)
		testutil.NoError(t, boot(a, b))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "runs", hello.calls(), [][]string(nil))
	})

	t.Run("should fail with invalid arguments", func(t *testing.T) {
		for name, value := range map[string]any{
			"number":        5,
			"list of other": []any{"hello", 1},
		} {
			a, b := newApp(t, map[string]any{command.ArgsConfigPath: value}, nil)
			ctx := map[string]any{command.ErrorFieldField: command.ArgsConfigPath}
			text := "unable to read command line: invalid command line arguments"
			t.Run(name+" boot", func(t *testing.T) {
				flamError(t, boot(a, b), command.ErrInvalidArgs, text, ctx)
			})
			t.Run(name+" run", func(t *testing.T) {
				flamError(t, run(a, b), command.ErrInvalidArgs, text, ctx)
			})
		}
	})

	t.Run("should keep the arguments read on the first use", func(t *testing.T) {
		hello, bye := newCmd("hello"), newCmd("bye")
		a, b := newApp(t, args("hello"), &bytes.Buffer{}, hello, bye)
		testutil.NoError(t, boot(a, b))
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.Set(command.ArgsConfigPath, []string{"bye"}) }))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "hello runs", hello.calls(), [][]string{{}})
		testutil.Equal(t, "bye runs", bye.calls(), [][]string(nil))
	})
}

func Test_Run(t *testing.T) {
	t.Run("should run the named command with its arguments", func(t *testing.T) {
		hello, bye := newCmd("hello"), newCmd("bye")
		out := &bytes.Buffer{}
		a, b := newApp(t, args("hello", "x", "y"), out, hello, bye)
		testutil.NoError(t, boot(a, b))
		testutil.Equal(t, "watchdog run", watchdogRun(t, a), false)
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "hello runs", hello.calls(), [][]string{{"x", "y"}})
		testutil.Equal(t, "bye runs", bye.calls(), [][]string(nil))
		testutil.Equal(t, "output", out.String(), "hello ran\n")
	})

	t.Run("should write to the standard output by default", func(t *testing.T) {
		hello := newCmd("hello")
		hello.fn = func(context.Context, []string, io.Writer) error { return nil }
		a, b := newApp(t, args("hello"), nil, hello)
		testutil.NoError(t, run(a, b))
		if len(hello.outs) != 1 || hello.outs[0] != io.Writer(os.Stdout) {
			t.Fatalf("outs = %v, want the standard output", hello.outs)
		}
	})

	t.Run("should return the error of the command", func(t *testing.T) {
		hello := newCmd("hello")
		hello.fn = func(context.Context, []string, io.Writer) error { return errBoom }
		a, b := newApp(t, args("hello"), nil, hello)
		err := run(a, b)
		testutil.Error(t, err, errBoom, "unable to run command: boom", map[string]any{command.ErrorFieldCommand: "hello"})
	})

	t.Run("should fail for an unknown command, listing the available ones", func(t *testing.T) {
		a, b := newApp(t, args("nope"), nil, newCmd("hello"), newCmd("bye"))
		err := run(a, b)
		testutil.Error(t, err, command.ErrUnknownCommand,
			`unable to run command: unknown command "nope", available: bye, hello, help`,
			map[string]any{command.ErrorFieldCommand: "nope", command.ErrorFieldCommands: []string{"bye", "hello", "help"}})
	})

	t.Run("should fail for an invalid command", func(t *testing.T) {
		for name, tc := range map[string]struct {
			commands []flam.ICommand
			sentinel error
			text     string
			ctx      map[string]any
		}{
			"nil": {
				[]flam.ICommand{nil}, command.ErrNilCommand,
				"unable to create command runner: nil command", map[string]any{},
			},
			"empty name": {
				[]flam.ICommand{newCmd("")}, command.ErrInvalidName,
				"unable to create command runner: invalid command name", map[string]any{command.ErrorFieldCommand: ""},
			},
			"flag name": {
				[]flam.ICommand{newCmd("-x")}, command.ErrInvalidName,
				"unable to create command runner: invalid command name", map[string]any{command.ErrorFieldCommand: "-x"},
			},
			"spaced name": {
				[]flam.ICommand{newCmd("a b")}, command.ErrInvalidName,
				"unable to create command runner: invalid command name", map[string]any{command.ErrorFieldCommand: "a b"},
			},
			"duplicate": {
				[]flam.ICommand{newCmd("hello"), newCmd("hello")}, command.ErrDuplicateCommand,
				"unable to create command runner: duplicate command name", map[string]any{command.ErrorFieldCommand: "hello"},
			},
			"help": {
				[]flam.ICommand{newCmd("help")}, command.ErrDuplicateCommand,
				"unable to create command runner: duplicate command name", map[string]any{command.ErrorFieldCommand: "help"},
			},
		} {
			t.Run(name, func(t *testing.T) {
				a := testutil.NewApp()
				b := command.NewBundle()
				testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
				testutil.NoError(t, a.RegisterBundle(b))
				for _, c := range tc.commands {
					testutil.NoError(t, a.Provide(func() flam.ICommand { return c }, dig.Group(command.CommandGroup)))
				}
				testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(values(t, args("help"))) }))
				flamError(t, run(a, b), tc.sentinel, tc.text, tc.ctx)
			})
		}
	})
}

func Test_Help(t *testing.T) {
	t.Run("should list the commands", func(t *testing.T) {
		out := &bytes.Buffer{}
		a, b := newApp(t, args("help"), out, newCmd("hello"), newCmd("migrate"))
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "output", out.String(), "Usage: <command> [arguments]\n\n"+
			"Commands:\n"+
			"  hello    the hello command\n"+
			"  help     list the commands, or describe the named one\n"+
			"  migrate  the migrate command\n")
	})

	t.Run("should list the help command alone", func(t *testing.T) {
		out := &bytes.Buffer{}
		a, b := newApp(t, args("help"), out)
		testutil.NoError(t, run(a, b))
		testutil.Equal(t, "output", out.String(), "Usage: <command> [arguments]\n\n"+
			"Commands:\n"+
			"  help  list the commands, or describe the named one\n")
	})

	t.Run("should describe the named command", func(t *testing.T) {
		for name, want := range map[string]string{
			"hello": "Usage: hello [arguments]\n\nthe hello command\n",
			"help":  "Usage: help [arguments]\n\nlist the commands, or describe the named one\n",
		} {
			out := &bytes.Buffer{}
			a, b := newApp(t, args("help", name, "ignored"), out, newCmd("hello"))
			testutil.NoError(t, run(a, b))
			testutil.Equal(t, name+" output", out.String(), want)
		}
	})

	t.Run("should fail for an unknown command", func(t *testing.T) {
		out := &bytes.Buffer{}
		a, b := newApp(t, args("help", "nope"), out, newCmd("hello"))
		err := run(a, b)
		testutil.Error(t, err, command.ErrUnknownCommand,
			`unable to run command: unknown command "nope", available: hello, help`,
			map[string]any{command.ErrorFieldCommand: "nope", command.ErrorFieldCommands: []string{"hello", "help"}})
		testutil.Equal(t, "output", out.String(), "")
	})

	t.Run("should fail when the output fails", func(t *testing.T) {
		a, b := newApp(t, args("help"), failingWriter{})
		err := run(a, b)
		testutil.Error(t, err, errBoom, "unable to run command: boom", map[string]any{command.ErrorFieldCommand: "help"})
	})
}

func Test_Signals(t *testing.T) {
	// waiting returns the command signaling its start on started, then
	// waiting for its context to be done, or for release, and returning
	// the context error.
	waiting := func(started chan<- struct{}, release <-chan struct{}) *cmd {
		c := newCmd("wait")
		c.fn = func(ctx context.Context, _ []string, _ io.Writer) error {
			started <- struct{}{}
			select {
			case <-ctx.Done():
			case <-release:
			}
			return ctx.Err()
		}
		return c
	}

	t.Run("should cancel the context of the command on a signal", func(t *testing.T) {
		for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
			started := make(chan struct{}, 1)
			a, b := newApp(t, args("wait"), nil, waiting(started, nil))
			result := make(chan error, 1)
			go func() { result <- run(a, b) }()
			<-started
			testutil.NoError(t, syscall.Kill(syscall.Getpid(), sig))
			err := <-result
			testutil.Error(t, err, context.Canceled, "unable to run command: context canceled",
				map[string]any{command.ErrorFieldCommand: "wait"})
		}
	})

	t.Run("should not cancel the context when the signals are disabled", func(t *testing.T) {
		started, release := make(chan struct{}, 1), make(chan struct{})
		close(release)
		vals := args("wait")
		vals[command.SignalsConfigPath] = false
		a, b := newApp(t, vals, nil, waiting(started, release))
		testutil.NoError(t, run(a, b))
	})

	t.Run("should not listen to the signals the program ignores", func(t *testing.T) {
		signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
		defer func() {
			// Reset keeps the signals ignored: listening to them, then
			// stopping, restores their default handling.
			restore := make(chan os.Signal, 1)
			signal.Notify(restore, syscall.SIGINT, syscall.SIGTERM)
			signal.Stop(restore)
			testutil.Equal(t, "ignored", signal.Ignored(syscall.SIGINT), false)
		}()
		started, release := make(chan struct{}, 1), make(chan struct{})
		a, b := newApp(t, args("wait"), nil, waiting(started, release))
		result := make(chan error, 1)
		go func() { result <- run(a, b) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		time.Sleep(50 * time.Millisecond)
		close(release)
		testutil.NoError(t, <-result)
	})
}

func Test_Application(t *testing.T) {
	// newFullApp returns an app with the command and watchdog bundles, in
	// the order given, a process, the hello command and the values.
	newFullApp := func(t *testing.T, vals map[string]any, commandFirst bool) (flam.IApp, *process, *cmd, *bytes.Buffer) {
		t.Helper()
		a := app.New(values(t, vals))
		bundles := []flam.IBundle{command.NewBundle(), watchdog.NewBundle()}
		if !commandFirst {
			bundles[0], bundles[1] = bundles[1], bundles[0]
		}
		for _, b := range bundles {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		p, hello, out := &process{}, newCmd("hello"), &bytes.Buffer{}
		testutil.NoError(t, a.Provide(func() flam.IWatchdogProcess { return p }, dig.Group(watchdog.ProcessGroup)))
		provide(t, a, out, hello)
		t.Cleanup(func() { _ = a.Close() })
		return a, p, hello, out
	}

	for name, commandFirst := range map[string]bool{"command first": true, "watchdog first": false} {
		t.Run("should run the command in place of the processes, "+name, func(t *testing.T) {
			a, p, hello, out := newFullApp(t, args("hello", "x"), commandFirst)
			testutil.NoError(t, a.Run())
			testutil.Equal(t, "process runs", p.count(), 0)
			testutil.Equal(t, "hello runs", hello.calls(), [][]string{{"x"}})
			testutil.Equal(t, "output", out.String(), "hello ran\n")
		})

		t.Run("should run the processes without command, "+name, func(t *testing.T) {
			a, p, hello, out := newFullApp(t, args(), commandFirst)
			testutil.NoError(t, a.Run())
			testutil.Equal(t, "process runs", p.count(), 1)
			testutil.Equal(t, "hello runs", hello.calls(), [][]string(nil))
			testutil.Equal(t, "output", out.String(), "")
		})
	}

	t.Run("should fail to run an unknown command, without running the processes", func(t *testing.T) {
		a, p, _, _ := newFullApp(t, args("nope"), true)
		if err := a.Run(); !errors.Is(err, command.ErrUnknownCommand) {
			t.Fatalf("Run() = %v, want ErrUnknownCommand", err)
		}
		testutil.Equal(t, "process runs", p.count(), 0)
	})

	t.Run("should fail to boot with invalid arguments", func(t *testing.T) {
		a, p, _, _ := newFullApp(t, map[string]any{command.ArgsConfigPath: 1}, true)
		if err := a.Run(); !errors.Is(err, command.ErrInvalidArgs) {
			t.Fatalf("Run() = %v, want ErrInvalidArgs", err)
		}
		testutil.Equal(t, "process runs", p.count(), 0)
	})
}
