package command

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// NewBundle returns the [flam.IBundle] of the command feature, providing
// to the application container the command line, read once from the
// configuration at [ArgsConfigPath], or else from the arguments of the
// program, and the runner of the commands of the [CommandGroup] group,
// along with the built-in [HelpCommand], writing to the io.Writer named
// [OutputName] in the container, or else to the standard output. The
// commands are only taken from the container when the command line names
// one.
//
// The bundle implements [flam.IBootableBundle]: when the application
// boots and the command line names a command, it stores false at
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog.RunConfigPath], so that
// the bundle of [github.com/cjdias/flam-in-go/pkg/flam/watchdog] does not
// run the long running processes, whatever the registration order of the
// two bundles. The boot fails when the value at ArgsConfigPath is not a
// list of strings.
//
// The bundle also implements [flam.IRunnableBundle]: when the application
// runs and the command line names a command, it runs that command, with a
// context cancelled on an interrupt or a SIGTERM signal, unless
// [SignalsConfigPath] holds false, and returns its error. The run fails
// when no command has the name, with an error listing the available ones,
// or when a command of the CommandGroup group is invalid. When the command
// line names no command, the run does nothing.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time checks that *bundleImpl implements IBootableBundle and
// IRunnableBundle
var (
	_ flam.IBootableBundle = (*bundleImpl)(nil)
	_ flam.IRunnableBundle = (*bundleImpl)(nil)
)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newLine).
		QueueProvider(newRunner).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.boot)
}

func (b *bundleImpl) Run(
	app flam.IApp,
) error {
	var l *line
	if err := app.Invoke(func(cl *line) { l = cl }); err != nil || !l.HasCommand() {
		return err
	}
	return app.Invoke(func(r *runner) error {
		return r.Run(l.args)
	})
}

// boot keeps the watchdog from running the long running processes when
// the command line names a command.
func (b *bundleImpl) boot(
	config flam.IConfig,
	l *line,
) error {
	if !l.HasCommand() {
		return nil
	}
	return config.Set(watchdog.RunConfigPath, false)
}
