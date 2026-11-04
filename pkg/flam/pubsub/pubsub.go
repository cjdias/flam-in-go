package pubsub

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// PubSub is an in-process publish/subscribe hub, delivering messages of
// type DATA to the subscriptions, identified by ids of type ID, whose
// pattern matches the topic they are published on.
//
// # Topics and patterns
//
// A topic is a sequence of non-empty segments joined by
// [TopicSeparator], such as "orders.eu.created". A pattern is a topic
// in which a segment may also be a wildcard: "*" matches exactly one
// segment, and "**", only allowed as the last segment, matches one or more
// segments. So "orders.*.created" matches "orders.eu.created", and
// "orders.**" matches "orders.eu" and "orders.eu.created" but not
// "orders". A segment cannot otherwise contain "*", in a topic or a
// pattern.
//
// # Delivery
//
// Delivery is asynchronous. Each subscription has its own goroutine and
// queue: [PubSub.Publish] appends the message to the queue of every
// matching subscription and returns without waiting for the handlers. A
// subscription handles its messages one at a time, in the order they were
// published; different subscriptions run concurrently. Queues are
// unbounded, so a handler slower than the publishers makes its queue grow.
// A message is delivered as is to every matching subscription: when DATA
// is a pointer, slice, map or contains one, the handlers share the data
// it refers to.
//
// A handler is called with the context given to Publish. When that context
// is done before the message is handled, the handler is not called and the
// context error is passed to the error callback instead. The error
// returned by a handler is passed to the error callback as is. A handler
// panic is recovered and reported as an [*flam.Error] wrapping
// [ErrHandlerPanic], and also the panic value when it is an error,
// with the "panic" context field holding the panic value. A handler ending
// its goroutine with [runtime.Goexit] is reported as an [*flam.Error]
// wrapping [ErrHandlerExit], and the subscription goes on with its
// next message on a new goroutine. Both errors hold the subscription id and
// the topic in the "id" and "topic" context fields. A nil error returned by
// a handler, including a nil pointer held by a non-nil error interface, is
// not a failure. The error callback may be nil, in which case such errors
// are dropped; a panic in the error callback itself is
// recovered and dropped, and a Goexit in it is survived the same way.
//
// # Lifecycle
//
// The zero value is an empty, open PubSub ready to use; [New]
// returns one too. A subscription stops receiving new messages once
// [PubSub.Unsubscribe] returns, and its goroutine ends after handling the
// messages already queued. [PubSub.Close] stops every subscription and
// waits until every queued message has been handled or skipped, after
// which the PubSub cannot be used anymore.
//
// All methods are safe for concurrent use, and a handler may call
// Subscribe, Unsubscribe and Publish, including for its own subscription.
// Close cannot be called from a handler or an error callback, as it would
// wait for that very callback to return: it returns an error instead.
type PubSub[ID comparable, DATA any] struct {
	mu            sync.RWMutex
	closed        bool
	subscriptions map[ID]*pubSubSubscription[DATA]
	workers       pubSubWorkers // the subscription goroutines
}

// compile time check that *PubSub implements IPubSub
var _ flam.IPubSub[int, any] = (*PubSub[int, any])(nil)

// New returns a new, open, empty [PubSub].
func New[ID comparable, DATA any]() *PubSub[ID, DATA] {
	return &PubSub[ID, DATA]{}
}

// Close stops every subscription and waits until every message already
// queued has been handled, or skipped because its context is done.
//
// Once Close is called, Subscribe, Unsubscribe and Publish fail with
// [ErrClosed]. Calling Close again is a no-op returning nil, once the
// first call has returned.
//
// Called from a handler or an error callback of this PubSub, Close returns
// an [*flam.Error] wrapping [ErrCloseFromHandler] and closes nothing, as
// it would otherwise wait for the very callback calling it. This is
// detected for the callback goroutine only: a callback waiting for another
// goroutine that calls Close still blocks forever.
func (p *PubSub[ID, DATA]) Close() error {
	if p.workers.isWorker(pubSubGoroutineID()) {
		return newErrCloseFromHandler(pubSubCloseMessage)
	}

	p.mu.Lock()
	if !p.closed {
		p.closed = true
		for _, s := range p.subscriptions {
			s.stop()
		}
		p.subscriptions = nil
	}
	p.mu.Unlock()

	p.workers.wait()
	return nil
}

// Subscribe registers a subscription, identified by id, delivering to
// handler the messages published on the topics matching pattern, as
// described in [PubSub]. Errors returned by handler, contexts done before
// handling, handler panics and handler Goexit calls are passed to onError,
// which may be nil.
//
// Subscribe returns an [*flam.Error] wrapping, from the first checked:
//   - [ErrInvalidPattern] when the pattern is malformed;
//   - [ErrNilHandler] when the handler is nil;
//   - [ErrInvalidID] when the id cannot be used as a map key;
//   - [ErrClosed] when the PubSub is closed;
//   - [ErrDuplicateID] when a subscription already uses the id.
//
// The "id" and "pattern" context fields hold the given id and pattern.
func (p *PubSub[ID, DATA]) Subscribe(
	id ID,
	pattern string,
	handler func(ctx context.Context, topic string, msg DATA) error,
	onError func(topic string, msg DATA, err error),
) error {
	segments, ok := pubSubSplit(pattern, true)
	if !ok {
		return newErrInvalidPattern(pubSubSubscribeMessage, id, pattern)
	}
	if handler == nil {
		return newErrNilHandler(pubSubSubscribeMessage, id, pattern)
	}
	if !pubSubValidID(id) {
		return newErrInvalidID(pubSubSubscribeMessage, id).
			With(ErrorFieldPattern, pattern)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return newErrClosed(pubSubSubscribeMessage).
			With(ErrorFieldID, id).
			With(ErrorFieldPattern, pattern)
	}
	if _, exists := p.subscriptions[id]; exists {
		return newErrDuplicateID(pubSubSubscribeMessage, id, pattern)
	}

	s := &pubSubSubscription[DATA]{
		id:      id,
		pattern: segments,
		handler: handler,
		onError: onError,
		workers: &p.workers,
	}
	s.wakeup = sync.NewCond(&s.mu)

	if p.subscriptions == nil {
		p.subscriptions = map[ID]*pubSubSubscription[DATA]{}
	}
	p.subscriptions[id] = s
	p.workers.start(s.run)
	return nil
}

// Unsubscribe removes the subscription identified by id. It receives no
// message published after Unsubscribe returns; the messages already queued
// are still handled, in the background, and [PubSub.Close] waits for
// them. Unsubscribe does not wait, so a handler may unsubscribe its own
// subscription.
//
// Unsubscribe returns an [*flam.Error] wrapping [ErrInvalidID] when
// the id cannot be used as a map key, [ErrClosed] when the PubSub is
// closed, or [ErrUnknownID] when no subscription uses the id, with
// the "id" context field holding the given id.
func (p *PubSub[ID, DATA]) Unsubscribe(
	id ID,
) error {
	if !pubSubValidID(id) {
		return newErrInvalidID(pubSubUnsubscribeMessage, id)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return newErrClosed(pubSubUnsubscribeMessage).
			With(ErrorFieldID, id)
	}
	s, exists := p.subscriptions[id]
	if !exists {
		return newErrUnknownID(pubSubUnsubscribeMessage, id)
	}

	delete(p.subscriptions, id)
	s.stop()
	return nil
}

// Publish queues msg for every subscription whose pattern matches topic,
// as described in [PubSub], and returns without waiting for the handlers.
// Publishing on a topic no subscription matches is not an error. A nil ctx
// is replaced with [context.Background].
//
// Publish returns an [*flam.Error], with the "topic" context field holding
// the given topic, wrapping, from the first checked:
//   - [ErrInvalidTopic] when the topic is malformed;
//   - the context error when ctx is already done, in which case the
//     message is not queued;
//   - [ErrClosed] when the PubSub is closed.
func (p *PubSub[ID, DATA]) Publish(
	ctx context.Context,
	topic string,
	msg DATA,
) error {
	segments, ok := pubSubSplit(topic, false)
	if !ok {
		return newErrInvalidTopic(pubSubPublishMessage, topic)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return newErrContextDone(pubSubPublishMessage, err, topic)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return newErrClosed(pubSubPublishMessage).
			With(ErrorFieldTopic, topic)
	}
	for _, s := range p.subscriptions {
		if pubSubMatch(s.pattern, segments) {
			s.enqueue(pubSubMessage[DATA]{ctx: ctx, topic: topic, msg: msg})
		}
	}
	return nil
}

// messages of the errors returned by the methods and passed to the error
// callbacks
const (
	pubSubCloseMessage       = "unable to close"
	pubSubSubscribeMessage   = "unable to subscribe"
	pubSubUnsubscribeMessage = "unable to unsubscribe"
	pubSubPublishMessage     = "unable to publish"
	pubSubHandleMessage      = "unable to handle message"
)

// pubSubSubscription is a registered subscription: its pattern, callbacks
// and queue of messages, handled in order by its goroutine (run).
type pubSubSubscription[DATA any] struct {
	id      any // for the error context
	pattern []string
	handler func(ctx context.Context, topic string, msg DATA) error
	onError func(topic string, msg DATA, err error)
	workers *pubSubWorkers // the PubSub ones, to replace an exited goroutine

	mu      sync.Mutex
	wakeup  *sync.Cond // signalled when queue grows or stopped is set
	queue   []pubSubMessage[DATA]
	stopped bool
}

// pubSubMessage is a published message waiting in a subscription queue.
type pubSubMessage[DATA any] struct {
	ctx   context.Context
	topic string
	msg   DATA
}

// enqueue appends a message to the queue. It is only called by Publish,
// under the PubSub read lock, while stop is only called under its write
// lock after removing the subscription, so it never sees a stopped
// subscription.
func (s *pubSubSubscription[DATA]) enqueue(
	m pubSubMessage[DATA],
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queue = append(s.queue, m)
	s.wakeup.Signal()
}

// stop makes the subscription refuse new messages and its goroutine end
// once the queue is empty.
func (s *pubSubSubscription[DATA]) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopped = true
	s.wakeup.Signal()
}

// run handles the queued messages in order until the subscription is
// stopped and its queue is empty. Panics of the callbacks are recovered by
// handle and report, so the only other way out is runtime.Goexit called by
// a callback, in which case a new goroutine takes over the queue.
func (s *pubSubSubscription[DATA]) run() {
	finished := false
	defer func() {
		if !finished {
			s.workers.start(s.run)
		}
	}()

	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.stopped {
			s.wakeup.Wait()
		}
		if len(s.queue) == 0 {
			s.mu.Unlock()
			finished = true
			return
		}
		m := s.queue[0]
		s.queue[0] = pubSubMessage[DATA]{} // release the message for the GC
		s.queue = s.queue[1:]
		s.mu.Unlock()

		s.deliver(m)
	}
}

// deliver calls the handler with the message, unless its context is done,
// and reports any failure to the error callback.
func (s *pubSubSubscription[DATA]) deliver(
	m pubSubMessage[DATA],
) {
	err := m.ctx.Err()
	if err == nil {
		err = s.handle(m)
	}
	s.report(m, err)
}

// handle calls the handler, turning a panic into an error. When the
// handler calls runtime.Goexit, the exit is reported here, as the goroutine
// ends before handle returns.
func (s *pubSubSubscription[DATA]) handle(
	m pubSubMessage[DATA],
) (err error) {
	returned := false
	defer func() {
		r := recover()
		switch {
		case r != nil:
			err = newErrHandlerPanic(pubSubHandleMessage, s.id, m.topic, r, debug.Stack())
		case !returned:
			s.report(m, newErrHandlerExit(pubSubHandleMessage, s.id, m.topic))
		}
	}()

	err = s.handler(m.ctx, m.topic, m.msg)
	returned = true
	return err
}

// report passes a delivery failure to the error callback, if any,
// dropping a panic of the callback. A nil err, including a nil pointer
// held by a non-nil error interface, is not a failure.
func (s *pubSubSubscription[DATA]) report(
	m pubSubMessage[DATA],
	err error,
) {
	if value.IsNil(err) || s.onError == nil {
		return
	}
	defer func() { _ = recover() }()
	s.onError(m.topic, m.msg, err)
}

// pubSubWorkers tracks the subscription goroutines of a PubSub: it waits for
// them on Close, and knows their goroutine ids so that Close can tell when
// it is called from one of them. Its zero value is ready to use.
type pubSubWorkers struct {
	running sync.WaitGroup
	mu      sync.Mutex
	ids     map[uint64]struct{}
}

// start runs fn on a new tracked goroutine.
func (w *pubSubWorkers) start(
	fn func(),
) {
	w.running.Go(func() {
		id := pubSubGoroutineID()
		w.mu.Lock()
		if w.ids == nil {
			w.ids = map[uint64]struct{}{}
		}
		w.ids[id] = struct{}{}
		w.mu.Unlock()

		defer func() { // also runs on runtime.Goexit
			w.mu.Lock()
			delete(w.ids, id)
			w.mu.Unlock()
		}()
		fn()
	})
}

// isWorker reports whether the goroutine with the given id is a tracked
// one.
func (w *pubSubWorkers) isWorker(
	id uint64,
) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.ids[id]
	return ok
}

// wait waits until every tracked goroutine has ended.
func (w *pubSubWorkers) wait() {
	w.running.Wait()
}

// pubSubGoroutineID returns the id of the calling goroutine. Go does not
// expose it on purpose, so it is read from the "goroutine N [...]" header
// of the goroutine stack trace, a format stable across Go versions. It is
// only used to recognise the subscription goroutines, never for anything
// else.
func pubSubGoroutineID() uint64 {
	var buf [64]byte
	header := buf[:runtime.Stack(buf[:], false)]
	header = bytes.TrimPrefix(header, []byte("goroutine "))
	if i := bytes.IndexByte(header, ' '); i >= 0 {
		header = header[:i]
	}
	id, _ := strconv.ParseUint(string(header), 10, 64)
	return id
}

// pubSubValidID reports whether id can be used as a map key: a value of a
// comparable type always can, but an interface value holding a slice, map,
// function or a struct containing one cannot, and would make the map panic.
func pubSubValidID(
	id any,
) bool {
	v := reflect.ValueOf(id)
	return !v.IsValid() || v.Comparable()
}

// pubSubSplit splits a topic, or a pattern when wildcards are allowed, into
// its segments, and reports whether it is well formed, as described in
// PubSub.
func pubSubSplit(
	s string,
	wildcards bool,
) ([]string, bool) {
	segments := strings.Split(s, TopicSeparator)
	for i, segment := range segments {
		switch {
		case segment == "":
			return nil, false
		case !strings.Contains(segment, "*"):
		case !wildcards:
			return nil, false
		case segment == "*":
		case segment == "**" && i == len(segments)-1:
		default:
			return nil, false
		}
	}
	return segments, true
}

// pubSubMatch reports whether the topic segments match the pattern
// segments.
func pubSubMatch(
	pattern []string,
	topic []string,
) bool {
	for i, segment := range pattern {
		if segment == "**" {
			return len(topic) > i // one or more remaining segments
		}
		if i >= len(topic) || (segment != "*" && segment != topic[i]) {
			return false
		}
	}
	return len(pattern) == len(topic)
}
