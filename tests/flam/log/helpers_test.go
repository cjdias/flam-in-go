package log

import (
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// streams
// ---------------------------------------------------------------------------

// written is an entry written to a fakeStream.
type written struct {
	timestamp time.Time
	level     flam.LogLevel
	broadcast bool
	channel   string
	message   string
	ctx       []flam.IBag
}

// fakeStream is a flam.ILogStream recording the entries written to it,
// failing to write with fail, and to close with closeErr.
type fakeStream struct {
	id       string
	fail     error
	closeErr error

	mu      sync.Mutex
	entries []written
	closed  bool
}

func (s *fakeStream) Id() string                   { return s.id }
func (s *fakeStream) GetLevel() flam.LogLevel      { return flam.LogDebug }
func (s *fakeStream) SetLevel(flam.LogLevel) error { return nil }
func (s *fakeStream) HasChannel(string) bool       { return true }
func (s *fakeStream) ListChannels() []string       { return nil }
func (s *fakeStream) AddChannel(string) error      { return nil }
func (s *fakeStream) RemoveChannel(string) error   { return nil }
func (s *fakeStream) RemoveAllChannels() error     { return nil }
func (s *fakeStream) record(w written) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, w)
	return s.fail
}
func (s *fakeStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.closeErr
}
func (s *fakeStream) isClosed() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.closed }
func (s *fakeStream) written() []written {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]written(nil), s.entries...)
}

func (s *fakeStream) Signal(ts time.Time, level flam.LogLevel, channel, message string, ctx ...flam.IBag) error {
	return s.record(written{timestamp: ts, level: level, channel: channel, message: message, ctx: ctx})
}

func (s *fakeStream) Broadcast(ts time.Time, level flam.LogLevel, message string, ctx ...flam.IBag) error {
	return s.record(written{timestamp: ts, level: level, broadcast: true, message: message, ctx: ctx})
}

// messages returns the messages written to the stream.
func (s *fakeStream) messages() []string {
	list := []string{}
	for _, w := range s.written() {
		list = append(list, w.message)
	}
	return list
}

// fakeStreams is a flam.ILogStreamCreator accepting the configurations of
// the "fake" driver, and keeping the streams it creates by id. A
// configuration holding "fail" fails the creation, one holding "write"
// gives a stream failing to write, and one holding "close" a stream
// failing to close.
type fakeStreams struct {
	mu      sync.Mutex
	created map[string]*fakeStream
}

func (c *fakeStreams) Accept(bag flam.IBag) bool { return bag.String("driver") == "fake" }

func (c *fakeStreams) Create(bag flam.IBag) (flam.ILogStream, error) {
	if bag.Bool("fail") {
		return nil, errBoom
	}
	s := &fakeStream{id: bag.String(factory.ConfigFieldId)}
	if bag.Bool("write") {
		s.fail = errBoom
	}
	if bag.Bool("close") {
		s.closeErr = errBoom
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.created == nil {
		c.created = map[string]*fakeStream{}
	}
	c.created[s.id] = s
	return s, nil
}

// get returns the stream created for the id, or nil.
func (c *fakeStreams) get(id string) *fakeStream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.created[id]
}

// lookupFailing is a flam.ILogStreamFactory whose Lookup finds nothing for
// the stored ids in missing, as if they were removed since listed.
type lookupFailing struct {
	flam.ILogStreamFactory
	missing map[string]bool
}

func (f *lookupFailing) Lookup(id string) (flam.ILogStream, bool) {
	if f.missing[id] {
		return nil, false
	}
	return f.ILogStreamFactory.Lookup(id)
}

// ---------------------------------------------------------------------------
// triggers
// ---------------------------------------------------------------------------

// fakeTriggers is a flam.ITriggerFactory recording the recurring triggers
// it creates, without running them, or failing with fail.
type fakeTriggers struct {
	mu      sync.Mutex
	fail    error
	created []*fakeTrigger
}

func (f *fakeTriggers) Close() error { return nil }

func (f *fakeTriggers) NewPulse(string, time.Duration, flam.TriggerCallback, ...flam.TriggerErrorHandler) (flam.ITrigger, error) {
	panic("unexpected NewPulse")
}

func (f *fakeTriggers) NewRecurring(
	id string,
	delay time.Duration,
	callback flam.TriggerCallback,
	onError ...flam.TriggerErrorHandler,
) (flam.ITrigger, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	t := &fakeTrigger{id: id, delay: delay, callback: callback, onError: onError, running: true}
	f.created = append(f.created, t)
	return t, nil
}

// all returns the triggers created so far.
func (f *fakeTriggers) all() []*fakeTrigger {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakeTrigger(nil), f.created...)
}

// running returns the delays of the running triggers.
func (f *fakeTriggers) running() []time.Duration {
	delays := []time.Duration{}
	for _, t := range f.all() {
		if t.IsRunning() {
			delays = append(delays, t.delay)
		}
	}
	return delays
}

// fakeTrigger is the flam.ITrigger created by fakeTriggers, whose Close
// fails with closeErr.
type fakeTrigger struct {
	id       string
	delay    time.Duration
	callback flam.TriggerCallback
	onError  []flam.TriggerErrorHandler
	closeErr error

	mu      sync.Mutex
	running bool
}

func (t *fakeTrigger) Id() string           { return t.id }
func (t *fakeTrigger) Delay() time.Duration { return t.delay }
func (t *fakeTrigger) IsRunning() bool      { t.mu.Lock(); defer t.mu.Unlock(); return t.running }
func (t *fakeTrigger) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	return t.closeErr
}
func (t *fakeTrigger) stop() { t.mu.Lock(); defer t.mu.Unlock(); t.running = false }

// fire calls the callback, and the handlers with its error, as a trigger
// does.
func (t *fakeTrigger) fire() {
	if err := t.callback(); err != nil {
		for _, h := range t.onError {
			h(err)
		}
	}
}

// ---------------------------------------------------------------------------
// application
// ---------------------------------------------------------------------------

var errBoom = errors.New("boom")

// now is the time of the clock of the test applications.
var now = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

// logApp is a testutil.App with the config and log bundles registered.
type logApp struct {
	*testutil.App
	cfg      flam.IConfig
	bundle   flam.IBundle
	clock    *testutil.Clock
	triggers *fakeTriggers
	streams  *fakeStreams
}

// newLogApp returns a logApp whose configuration holds the defaults, with
// the triggers, unless nil, and the fake stream creator provided. The
// options, run before the log bundle is registered, may decorate the
// container.
func newLogApp(
	t *testing.T,
	defaults container.Bag,
	triggers *fakeTriggers,
	options ...func(*testutil.App),
) *logApp {
	t.Helper()
	a := &logApp{
		App:      testutil.NewApp(),
		bundle:   log.NewBundle(),
		clock:    testutil.NewClock(now),
		triggers: triggers,
		streams:  &fakeStreams{},
	}
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.Provide(func() flam.IClock { return a.clock }))
	if triggers != nil {
		testutil.NoError(t, a.Provide(func() flam.ITriggerFactory { return triggers }))
	}
	testutil.NoError(t, a.Provide(
		func() flam.ILogStreamCreator { return a.streams },
		dig.Group(log.StreamCreatorGroup),
	))
	for _, option := range options {
		option(a.App)
	}
	testutil.NoError(t, a.RegisterBundle(a.bundle))
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) { a.cfg = c }))
	testutil.NoError(t, a.cfg.AddDefaults(defaults))
	return a
}

// boot boots the log bundle.
func (a *logApp) boot() error {
	return a.bundle.(flam.IBootableBundle).Boot(a)
}

// close closes the log bundle.
func (a *logApp) close() error {
	return a.bundle.(flam.IClosableBundle).Close(a)
}

// logger returns the logger of the container.
func (a *logApp) logger(t *testing.T) flam.ILogger {
	t.Helper()
	var l flam.ILogger
	testutil.NoError(t, a.Invoke(func(logger flam.ILogger) { l = logger }))
	return l
}

// factory returns the stream factory of the container.
func (a *logApp) factory(t *testing.T) flam.ILogStreamFactory {
	t.Helper()
	var f flam.ILogStreamFactory
	testutil.NoError(t, a.Invoke(func(streams flam.ILogStreamFactory) { f = streams }))
	return f
}

// observing reports whether the flusher observes the period.
func (a *logApp) observing() bool {
	return a.cfg.HasObserver(log.FlusherTriggerID, log.FlusherPeriodConfigPath)
}

// unset leaves the flush period to its default, in defaults.
const unset = "unset"

// defaults returns defaults configuring a fake stream per id, with the
// flags set, and holding the flush period, zero when nil, so that the
// logger is not flushed periodically, unless unset.
func defaults(t *testing.T, period any, ids map[string][]string) container.Bag {
	t.Helper()
	bag := container.Bag{}
	for id, flags := range ids {
		stream := container.Bag{"driver": "fake"}
		for _, flag := range flags {
			stream[flag] = true
		}
		testutil.NoError(t, bag.Set(log.StreamConfigPath+"."+id, stream))
	}
	switch period {
	case unset:
	case nil:
		testutil.NoError(t, bag.Set(log.FlusherPeriodConfigPath, "0s"))
	default:
		testutil.NoError(t, bag.Set(log.FlusherPeriodConfigPath, period))
	}
	return bag
}

// joined returns the errors joined in err.
func joined(t *testing.T, err error) []error {
	t.Helper()
	j, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("error = %T %v, want joined errors", err, err)
	}
	return j.Unwrap()
}
