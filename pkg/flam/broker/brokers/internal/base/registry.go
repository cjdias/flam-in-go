package base

import (
	"context"
	"errors"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
)

// NewRegistry returns the registry of the subscriptions of the broker of
// the id.
func NewRegistry(
	id string,
) *Registry {
	return &Registry{id: id, subs: map[*Subscription]struct{}{}}
}

// Registry holds the subscriptions of a broker, refusing new ones once
// closed, and closing those under way when it closes. It is safe for
// concurrent use.
type Registry struct {
	id     string
	mu     sync.Mutex
	closed bool
	subs   map[*Subscription]struct{}
}

// CheckPublish returns the error, of the message, wrapping
// broker.ErrClosed once the registry is closed, or broker.ErrInvalidTopic
// when the topic is empty, holding the id and the topic, or nil.
func (r *Registry) CheckPublish(
	message string,
	topic string,
) error {
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	switch {
	case closed:
		return NewErrTopic(broker.ErrClosed, message, r.id, topic, nil)
	case topic == "":
		return NewErrTopic(broker.ErrInvalidTopic, message, r.id, topic, nil)
	}
	return nil
}

// CheckSubscribe returns the error of CheckPublish, or the one wrapping
// broker.ErrNilHandler when the handler is nil, or nil.
func (r *Registry) CheckSubscribe(
	message string,
	topic string,
	handler flam.BrokerHandler,
) error {
	if err := r.CheckPublish(message, topic); err != nil {
		return err
	}
	if handler == nil {
		return NewErrTopic(broker.ErrNilHandler, message, r.id, topic, nil)
	}
	return nil
}

// Start runs the subscription, calling run with its context in a
// goroutine of its own, then, once run returned, release, the error of
// run ending the subscription and the one of release being returned by
// its Close, and returns it. When the registry is closed, it releases the
// subscription at once, and fails with the error, of the message,
// wrapping broker.ErrClosed.
func (r *Registry) Start(
	message string,
	s *Subscription,
	run func(ctx context.Context) error,
	release func() error,
) (flam.IBrokerSubscription, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		s.cancel()
		_ = release()
		return nil, NewErrTopic(broker.ErrClosed, message, r.id, s.topic, nil)
	}
	r.subs[s] = struct{}{}
	r.mu.Unlock()

	go func() {
		err := run(s.ctx)
		s.cancel()
		s.closeErr = release()
		s.err = err
		r.mu.Lock()
		delete(r.subs, s)
		r.mu.Unlock()
		close(s.done)
	}()
	return s, nil
}

// Close closes the registry, then the subscriptions under way, returning
// the errors of their Close joined, or nil. Closing it again does
// nothing.
func (r *Registry) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	subs := make([]*Subscription, 0, len(r.subs))
	for s := range r.subs {
		subs = append(subs, s)
	}
	r.mu.Unlock()

	errs := make([]error, 0, len(subs))
	for _, s := range subs {
		errs = append(errs, s.Close())
	}
	return errors.Join(errs...)
}

// NewSubscription returns the subscription to the topic, its context
// being done once the context is, a nil context never being done, or once
// it ends. It runs once given to the Start method of a registry.
func NewSubscription(
	ctx context.Context,
	topic string,
) *Subscription {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Subscription{topic: topic, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

// Subscription is the [flam.IBrokerSubscription] of the drivers.
type Subscription struct {
	topic    string
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	err      error
	closeErr error
}

// compile time check that *Subscription implements IBrokerSubscription
var _ flam.IBrokerSubscription = (*Subscription)(nil)

// Context returns the context of the subscription, done once it ends,
// given to its handler.
func (s *Subscription) Context() context.Context {
	return s.ctx
}

func (s *Subscription) Topic() string {
	return s.topic
}

func (s *Subscription) Done() <-chan struct{} {
	return s.done
}

func (s *Subscription) Err() error {
	select {
	case <-s.done:
		return s.err
	default:
		return nil
	}
}

// Close ends the subscription, waiting for the handler under way, and
// returns the error of its release. As it waits for the handler, a
// handler must not close its own subscription but in another goroutine.
func (s *Subscription) Close() error {
	s.cancel()
	<-s.done
	return s.closeErr
}
