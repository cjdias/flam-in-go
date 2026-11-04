package leader

import (
	"context"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewProcess returns the [flam.ILeaderProcess] of the id campaigning for
// the lock of the key of the locker, acquired for the ttl, and running the
// function while it holds it, as the leader.
//
// Its Run campaigns, waiting for the lock with the Acquire method of the
// locker, then runs the function with a context cancelled once the lock is
// lost, the context of Run is done or the process is closed, holding the
// fence of the lock, read with [FenceOf]. Once the function returns, the
// process releases the lock, ignoring the error of the release, as the
// lock expires anyway, then campaigns again, unless the function returned
// an error, which Run then returns, or the context of Run is done or the
// process is closed, Run then returning nil. Run returns the errors of the
// locker other than those of its context being done. A function returning
// nil before its context is done gives the leadership up, the process
// campaigning again at once, so that it usually blocks until its context
// is done.
//
// Run fails with ErrRunning when the process runs already, and returns at
// once once the process is closed. A panic of the function releases the
// lock and goes through Run, so that the kennel runs it again. Closing the
// process does not close the locker, which its factory owns.
//
// NewProcess fails with ErrNilLocker, ErrNilRun, ErrInvalidKey for an
// empty key or ErrInvalidTTL for a ttl shorter than 1ms, holding the id and
// the key.
func NewProcess(
	id string,
	locker flam.ILocker,
	key string,
	ttl time.Duration,
	run func(ctx context.Context) error,
) (flam.ILeaderProcess, error) {
	var sentinel error
	switch {
	case locker == nil:
		sentinel = ErrNilLocker
	case run == nil:
		sentinel = ErrNilRun
	case key == "":
		sentinel = ErrInvalidKey
	case ttl < time.Millisecond:
		sentinel = ErrInvalidTTL
	default:
		return &process{id: id, locker: locker, key: key, ttl: ttl, run: run, stop: make(chan struct{})}, nil
	}
	return nil, newErr(sentinel, processNewMessage, id, key)
}

// FenceOf returns the fencing token of the lock of the leadership the
// context is of, as given to the function of a process of [NewProcess],
// and false for any other context.
func FenceOf(
	ctx context.Context,
) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	fence, ok := ctx.Value(fenceKey{}).(int64)
	return fence, ok
}

// process is the [flam.ILeaderProcess] returned by [NewProcess].
type process struct {
	id     string
	locker flam.ILocker
	key    string
	ttl    time.Duration
	run    func(ctx context.Context) error

	stop    chan struct{}
	mu      sync.Mutex // guards the fields below
	running bool
	closed  bool
	leader  bool
}

func (p *process) Id() string {
	return p.id
}

func (p *process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *process) IsLeader() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.leader
}

// Run campaigns for the lock and runs the function while it holds it,
// until the context is done, the process is closed or the function
// fails. A nil context never ends.
func (p *process) Run(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	switch {
	case p.closed:
		p.mu.Unlock()
		return nil
	case p.running:
		p.mu.Unlock()
		return newErr(ErrRunning, processRunMessage, p.id, p.key)
	}
	p.running = true
	p.mu.Unlock()
	defer p.setRunning(false)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-p.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	for {
		lock, err := p.locker.Acquire(ctx, p.key, p.ttl)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := p.lead(ctx, lock); err != nil {
			return err
		}
	}
}

// Close stops the process, making a Run under way cancel the context of
// the function, release the lock and return, and the later ones return at
// once. It does not wait for the function to return.
func (p *process) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		close(p.stop)
	}
	return nil
}

// Messages of the errors returned by NewProcess and Run.
const (
	processNewMessage = "unable to create leader process"
	processRunMessage = "unable to run leader process"
)

// fenceKey is the key of the fence in the context of the function.
type fenceKey struct{}

// lead runs the function while the lock is held, then releases it,
// returning the error of the function.
func (p *process) lead(
	ctx context.Context,
	lock flam.ILock,
) error {
	p.setLeader(true)
	runCtx, cancel := context.WithCancel(context.WithValue(ctx, fenceKey{}, lock.Fence()))
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		select {
		case <-lock.Lost():
			p.setLeader(false)
			cancel()
		case <-runCtx.Done():
		}
	}()
	defer func() {
		p.setLeader(false)
		cancel()
		<-watched
		_ = lock.Release(context.WithoutCancel(ctx))
	}()
	return p.run(runCtx)
}

// setRunning sets whether Run is under way.
func (p *process) setRunning(
	running bool,
) {
	p.mu.Lock()
	p.running = running
	p.mu.Unlock()
}

// setLeader sets whether the process holds the leadership.
func (p *process) setLeader(
	leader bool,
) {
	p.mu.Lock()
	p.leader = leader
	p.mu.Unlock()
}
