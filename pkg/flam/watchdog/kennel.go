package watchdog

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// kennelArgs are the dependencies of the kennel, taken from the
// container. The group tag repeats ProcessGroup, as a struct tag cannot
// use a constant.
type kennelArgs struct {
	dig.In

	Config    flam.IConfig
	Clock     flam.IClock
	Loggers   flam.IWatchdogLoggerAdaptorFactory
	Processes []flam.IWatchdogProcess `group:"flam.watchdog.processes"`
}

// newKennel returns the kennel holding the processes of the ProcessGroup
// group, each configured at ProcessConfigPath, added in id order, as the
// container shuffles the group, waiting between the restarts of a process
// as configured at RestartDelayConfigPath and RestartMaxDelayConfigPath,
// and draining on a signal for the delay at DrainDelayConfigPath. It is a
// provider registered by the bundle returned from [NewBundle].
//
// It never fails, as the container would call it again, and so close the
// processes again: when the delays are invalid, or at the first process
// that cannot be added, it closes the kennel and every process of the
// group, keeping the error, joined with those of the processes failing to
// close, for the provider of the [flam.IKennel] to return, and for the
// bundle to report once.
func newKennel(
	args kennelArgs,
) *kennel {
	k := &kennel{
		clock:   args.Clock,
		loggers: args.Loggers,
		entries: map[string]*entry{},
	}
	k.ctx, k.cancel = context.WithCancel(context.Background())

	processes := slices.SortedStableFunc(slices.Values(args.Processes), func(a, b flam.IWatchdogProcess) int {
		return strings.Compare(processId(a), processId(b))
	})
	err := k.configureDelays(args.Config)
	for i := 0; err == nil && i < len(processes); i++ {
		err = k.addConfigured(args.Config, processes[i])
	}
	if err != nil {
		k.closed = true
		k.cancel()
		k.err = errors.Join(err, closeProcesses(processes))
	}
	return k
}

// kennel is the [flam.IKennel] provided by the bundle, running each of its
// active processes under a watchdog, while mu guards its entries and
// state.
type kennel struct {
	clock    flam.IClock
	loggers  flam.IWatchdogLoggerAdaptorFactory
	delay    time.Duration
	maxDelay time.Duration
	drain    time.Duration      // the delay of the drain on a signal
	err      error              // the error of the creation, if any
	ctx      context.Context    // the context of the runs, done on close
	cancel   context.CancelFunc // cancels ctx

	mu       sync.Mutex
	entries  map[string]*entry
	running  bool
	draining bool // whether a signal started the shutdown of the run
	closed   bool
	reported bool // whether the error of the creation was reported
}

// entry is a process of the kennel, with its logger adaptor, if any, and
// whether it is active.
type entry struct {
	process flam.IWatchdogProcess
	logger  flam.IWatchdogLoggerAdaptor
	active  bool
}

// compile time check that *kennel implements IKennel
var _ flam.IKennel = (*kennel)(nil)

func (k *kennel) Id() string {
	return KennelId
}

func (k *kennel) Available() []string {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.ids()
}

func (k *kennel) Has(
	id string,
) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	_, ok := k.entries[id]
	return ok
}

// Add adds the process, failing with ErrNilProcess for a nil one,
// ErrInvalidID for an empty id, ErrIDMismatch when its id is not the id
// given, ErrLogger when the logger adaptor cannot be taken from the
// factory, ErrClosed once the kennel is closed, ErrRunning while it runs
// its processes, and ErrDuplicateProcess when the id is in use. The
// process is closed by the kennel only once added. The logger adaptor is
// taken from the factory without holding the kennel, as the factory may
// create it.
func (k *kennel) Add(
	id string,
	active bool,
	loggerAdaptorId string,
	process flam.IWatchdogProcess,
) error {
	switch {
	case value.IsNil(process):
		return newErr(ErrNilProcess, kennelAddMessage, id)
	case id == "":
		return newErr(ErrInvalidID, kennelAddMessage, id)
	case process.Id() != id:
		return newErr(ErrIDMismatch, kennelAddMessage, id).
			With(ErrorFieldProcess, process.Id())
	}
	var logger flam.IWatchdogLoggerAdaptor
	if loggerAdaptorId != "" {
		var err error
		if logger, err = k.loggers.Get(loggerAdaptorId); err != nil {
			return newErrJoined(ErrLogger, kennelAddMessage, id, err).
				With(ErrorFieldLogger, loggerAdaptorId)
		}
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	switch _, ok := k.entries[id]; {
	case k.closed:
		return newErr(ErrClosed, kennelAddMessage, id)
	case k.running:
		return newErr(ErrRunning, kennelAddMessage, id)
	case ok:
		return newErr(ErrDuplicateProcess, kennelAddMessage, id)
	}
	k.entries[id] = &entry{process: process, logger: logger, active: active}
	return nil
}

func (k *kennel) IsActive(
	id string,
) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	e, ok := k.entries[id]
	return ok && e.active
}

func (k *kennel) Process(
	id string,
) (flam.IWatchdogProcess, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()

	e, ok := k.entries[id]
	if !ok {
		return nil, false
	}
	return e.process, true
}

func (k *kennel) IsRunning() bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.running
}

// IsDraining reports whether a signal received while the kennel ran its
// processes started its shutdown: it stays true from the signal on,
// through the drain delay and the closing of the processes, as the kennel
// never runs again once closed.
func (k *kennel) IsDraining() bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.draining
}

// Close closes the kennel, as the close of the application does.
func (k *kennel) Close() error {
	return k.close()
}

// Activate fails with ErrUnknownProcess for an unknown process, and with
// ErrRunning while the kennel runs its processes.
func (k *kennel) Activate(
	id string,
) error {
	return k.setActive(id, true, kennelActivateMessage)
}

// Deactivate fails with ErrUnknownProcess for an unknown process, and with
// ErrRunning while the kennel runs its processes.
func (k *kennel) Deactivate(
	id string,
) error {
	return k.setActive(id, false, kennelDeactivateMessage)
}

// Messages of the errors returned by the kennel.
const (
	kennelAddMessage        = "unable to add watchdog process"
	kennelActivateMessage   = "unable to activate watchdog process"
	kennelDeactivateMessage = "unable to deactivate watchdog process"
	kennelCreateMessage     = "unable to create watchdog kennel"
	kennelRunMessage        = "unable to run watchdog kennel"
	kennelCloseMessage      = "unable to close watchdog kennel"
	kennelWatchMessage      = "unable to run watchdog process"
)

// ids returns the ids of the processes, sorted. The caller holds mu.
func (k *kennel) ids() []string {
	ids := make([]string, 0, len(k.entries))
	for id := range k.entries {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// configureDelays sets the restart delays of the kennel from the
// configuration at RestartDelayConfigPath and RestartMaxDelayConfigPath,
// or their defaults, the maximum default being raised to the delay when
// shorter, and its drain delay from the configuration at
// DrainDelayConfigPath, or its default, failing with ErrInvalidField for
// a value that is not a non-negative duration, or a maximum set shorter
// than the delay.
func (k *kennel) configureDelays(
	config flam.IConfig,
) error {
	var err error
	if k.drain, err = configDuration(config, DrainDelayConfigPath, defaultDrainDelay); err != nil {
		return err
	}
	if k.delay, err = configDuration(config, RestartDelayConfigPath, defaultRestartDelay); err != nil {
		return err
	}
	if k.maxDelay, err = configDuration(config, RestartMaxDelayConfigPath, max(defaultRestartMaxDelay, k.delay)); err != nil {
		return err
	}
	if k.maxDelay < k.delay {
		return newErr(ErrInvalidField, kennelCreateMessage, KennelId).
			With(ErrorFieldField, RestartMaxDelayConfigPath)
	}
	return nil
}

// configDuration returns the duration of the configuration at the path,
// or the default when unset, failing with ErrInvalidField for a value
// that is not a non-negative duration, such as an integer or "1 minute".
func configDuration(
	config flam.IConfig,
	path string,
	def time.Duration,
) (time.Duration, error) {
	if config.Get(path) == nil {
		return def, nil
	}
	const invalid = time.Duration(-1)
	if d := config.Duration(path, invalid); d >= 0 {
		return d, nil
	}
	return 0, newErr(ErrInvalidField, kennelCreateMessage, KennelId).
		With(ErrorFieldField, path)
}

// addConfigured adds the process with the active and logger fields of its
// configuration at ProcessConfigPath, or their defaults, failing with
// ErrInvalidID for an id that is empty or holds a dot, which would make
// it a nested path of the configuration, and with ErrInvalidField for a
// configuration that is not a bag, or an invalid field.
func (k *kennel) addConfigured(
	config flam.IConfig,
	process flam.IWatchdogProcess,
) error {
	if value.IsNil(process) {
		return k.Add("", false, "", process)
	}
	id := process.Id()
	if id == "" || strings.Contains(id, ".") {
		return newErr(ErrInvalidID, kennelCreateMessage, id)
	}
	path := ProcessConfigPath + "." + id
	if raw := config.Get(path); raw != nil {
		if _, ok := raw.(flam.IBag); !ok {
			return newErr(ErrInvalidField, kennelCreateMessage, id).
				With(ErrorFieldField, path)
		}
	}
	bag := config.Bag(path, container.Bag{})

	active, ok := bag.Get(ConfigFieldActive, defaultActive).(bool)
	if !ok {
		return newErr(ErrInvalidField, kennelCreateMessage, id).
			With(ErrorFieldField, ConfigFieldActive)
	}
	val := bag.Get(ConfigFieldLogger)
	if val == nil {
		val = config.Get(DefaultLoggerConfigPath, "")
	}
	logger, ok := val.(string)
	if !ok {
		return newErr(ErrInvalidField, kennelCreateMessage, id).
			With(ErrorFieldField, ConfigFieldLogger)
	}
	return k.Add(id, active, logger, process)
}

// processId returns the id of the process, or an empty id for a nil one.
func processId(
	process flam.IWatchdogProcess,
) string {
	if value.IsNil(process) {
		return ""
	}
	return process.Id()
}

// setActive sets whether the process of the id is active.
func (k *kennel) setActive(
	id string,
	active bool,
	message string,
) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	e, ok := k.entries[id]
	switch {
	case !ok:
		return newErr(ErrUnknownProcess, message, id)
	case k.running:
		return newErr(ErrRunning, message, id)
	}
	e.active = active
	return nil
}

// creationErr returns the error of the creation of the kennel the first
// time it is called, and nil afterwards, so that the run and the close of
// the application do not both report it.
func (k *kennel) creationErr() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.reported {
		return nil
	}
	k.reported = true
	return k.err
}

// run runs each active process under a watchdog, and waits for them to
// end, returning their errors joined. When signals is true, the first
// interrupt or SIGTERM signal received meanwhile makes the kennel drain,
// then closes it, and so its processes, the error of the closing joining
// theirs, while the next one cuts the drain short, and the one following
// the drain is handled as if the kennel had not been listening, ending
// the program, so that a process failing to end can be killed. A signal
// the program started ignoring, such as the interrupt of a background
// job, stays ignored. It fails with ErrClosed once the kennel is closed.
// The run of the application calls it from one goroutine.
func (k *kennel) run(
	signals bool,
) error {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return newErr(ErrClosed, kennelRunMessage, KennelId)
	}
	var ids []string
	var watched []*entry
	for _, id := range k.ids() {
		if e := k.entries[id]; e.active {
			ids, watched = append(ids, id), append(watched, e)
		}
	}
	if len(ids) == 0 {
		k.mu.Unlock()
		return nil
	}
	k.running = true
	k.mu.Unlock()
	defer k.setRunning(false)

	var closeErr error
	stop := func() {}
	if listened := listenedSignals(); signals && len(listened) > 0 {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, listened...)
		done, stopped := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(stopped)
			select {
			case <-sigs:
				if k.drainFor(sigs, done) {
					signal.Stop(sigs)
					closeErr = k.close()
				}
			case <-done:
			}
		}()
		stop = func() {
			signal.Stop(sigs)
			close(done)
			<-stopped
		}
	}

	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Go(func() {
			errs[i] = k.watch(id, watched[i])
		})
	}
	wg.Wait()
	stop()
	return errors.Join(append(errs, closeErr)...)
}

// listenedSignals returns the signals closing the kennel while it runs,
// the interrupt and SIGTERM, but those the program started ignoring, as
// listening to a signal stops ignoring it.
func listenedSignals() []os.Signal {
	var listened []os.Signal
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		if !signal.Ignored(sig) {
			listened = append(listened, sig)
		}
	}
	return listened
}

// drainFor marks the kennel draining, then waits for its drain delay, the
// processes still running meanwhile, unless a signal is received on sigs,
// cutting the drain short, reporting whether the kennel is to be closed:
// false when done is closed first, as the processes all ended.
func (k *kennel) drainFor(
	sigs <-chan os.Signal,
	done <-chan struct{},
) bool {
	k.mu.Lock()
	k.draining = true
	k.mu.Unlock()

	if k.drain == 0 {
		return true
	}
	timer := k.clock.NewTimer(k.drain)
	defer timer.Stop()
	select {
	case <-timer.C():
	case <-sigs:
	case <-done:
		return false
	}
	return true
}

// setRunning sets whether the kennel runs its processes.
func (k *kennel) setRunning(
	running bool,
) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.running = running
}

// watch runs the process of the entry, unless the kennel is closed,
// running it again, after a delay, when it panics, until the kennel is
// closed, and reporting to its logger adaptor, if any, its start, its
// failures and its end. It returns the error the process returned, or nil
// when the kennel got closed after a panic, as the panic was reported and
// the end of the process was asked for.
//
// The delay of the first restart is the delay of the kennel, doubled at
// each following one, from minRestartDelay at least, up to its maximum
// delay, and back to the delay of the kennel after a run lasting the
// maximum delay at least.
func (k *kennel) watch(
	id string,
	e *entry,
) error {
	var err error
	started, panicked, restarted := false, false, false
	for delay := time.Duration(0); k.wait(delay); restarted = true {
		if !started {
			e.start(id)
			started = true
		}
		begin := k.clock.Now()
		if panicked, err = e.run(k.ctx, id); err != nil {
			e.error(id, err)
		}
		if !panicked {
			break
		}
		delay = k.nextDelay(delay, k.clock.Since(begin), restarted)
	}
	if started {
		e.done(id)
	}
	if panicked {
		return nil
	}
	return err
}

// nextDelay returns the delay before the restart following a panic, from
// the delay before the run that panicked, the time it ran, and whether it
// was a restart: the delay of the kennel for the first restart, or after
// a long run, and else the double of the delay, from minRestartDelay at
// least, so that a delay of 0 does not restart a process at once again
// and again, up to the maximum delay.
func (k *kennel) nextDelay(
	delay time.Duration,
	ran time.Duration,
	restarted bool,
) time.Duration {
	if !restarted || ran >= k.maxDelay {
		return k.delay
	}
	return min(max(2*delay, minRestartDelay), k.maxDelay)
}

// wait waits for the delay, returning false when the kennel is, or gets,
// closed.
func (k *kennel) wait(
	delay time.Duration,
) bool {
	if delay > 0 {
		timer := k.clock.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C():
		case <-k.ctx.Done():
		}
	}
	return !k.isClosed()
}

// isClosed reports whether the kennel is closed.
func (k *kennel) isClosed() bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.closed
}

// close closes the kennel, cancelling the context of the runs, and every
// process, returning the errors of the processes failing to close joined.
// Closing it again is a no-op returning nil.
func (k *kennel) close() error {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return nil
	}
	k.closed = true
	k.cancel()
	processes := make([]flam.IWatchdogProcess, 0, len(k.entries))
	for _, id := range k.ids() {
		processes = append(processes, k.entries[id].process)
	}
	k.mu.Unlock()

	return closeProcesses(processes)
}

// closeProcesses closes the processes concurrently, so that one blocking
// does not hold the others, skipping the nil ones, and returns, in order,
// the errors of those failing to close joined.
func closeProcesses(
	processes []flam.IWatchdogProcess,
) error {
	errs := make([]error, len(processes))
	var wg sync.WaitGroup
	for i, process := range processes {
		if value.IsNil(process) {
			continue
		}
		wg.Go(func() {
			if err := process.Close(); err != nil {
				errs[i] = newErrJoined(ErrClose, kennelCloseMessage, process.Id(), err)
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// run runs the process with the context, returning its error, or true and
// the error of its panic, holding the stack of the panic.
func (e *entry) run(
	ctx context.Context,
	id string,
) (panicked bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			panicked, err = true, newErrPanic(kennelWatchMessage, id, r, debug.Stack())
		}
	}()
	return false, e.process.Run(ctx)
}

// start reports the start of the process to the logger adaptor, if any.
func (e *entry) start(
	id string,
) {
	e.report(func(logger flam.IWatchdogLoggerAdaptor) { logger.Start(id) })
}

// error reports the failure of the process to the logger adaptor, if any.
func (e *entry) error(
	id string,
	err error,
) {
	e.report(func(logger flam.IWatchdogLoggerAdaptor) { logger.Error(id, err) })
}

// done reports the end of the process to the logger adaptor, if any.
func (e *entry) done(
	id string,
) {
	e.report(func(logger flam.IWatchdogLoggerAdaptor) { logger.Done(id) })
}

// report reports to the logger adaptor, if any, dropping its panic, so
// that a failing logger adaptor does not end the program the watchdog
// keeps running.
func (e *entry) report(
	fn func(logger flam.IWatchdogLoggerAdaptor),
) {
	if e.logger == nil {
		return
	}
	defer func() { _ = recover() }()
	fn(e.logger)
}
