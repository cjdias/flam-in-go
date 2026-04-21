package flam

import (
	"sync"
)

type PubSubHandler[I comparable, C comparable] func(channel C, data ...any) error

type PubSub[I comparable, C comparable] interface {
	Subscribe(id I, channel C, handler PubSubHandler[I, C]) error
	Unsubscribe(id I, channel C) error
	Publish(channel C, data ...any) error
}

type pubsub[I comparable, C comparable] struct {
	mu       sync.Mutex
	handlers map[C]map[I]PubSubHandler[I, C]
}

var _ PubSub[string, string] = (*pubsub[string, string])(nil)

func NewPubSub[I comparable, C comparable]() PubSub[I, C] {
	return &pubsub[I, C]{
		handlers: map[C]map[I]PubSubHandler[I, C]{},
	}
}

func (ps *pubsub[I, C]) Subscribe(
	id I,
	channel C,
	handler PubSubHandler[I, C],
) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// ensure channel map exists
	if _, ok := ps.handlers[channel]; !ok {
		ps.handlers[channel] = map[I]PubSubHandler[I, C]{}
	}

	// check if id already exists in channel
	if _, ok := ps.handlers[channel][id]; ok {
		return newErrDuplicateSubscription(id, channel)
	}

	ps.handlers[channel][id] = handler

	return nil
}

func (ps *pubsub[I, C]) Unsubscribe(
	id I,
	channel C,
) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// check if channel exists
	if subs, ok := ps.handlers[channel]; ok {
		// check if id exists in channel
		if _, ok = ps.handlers[channel][id]; ok {
			delete(subs, id)

			// Clean up empty channel maps to prevent memory leaks
			if len(subs) == 0 {
				delete(ps.handlers, channel)
			}

			return nil
		}
	}

	return newErrSubscriptionNotFound(id, channel)
}

func (ps *pubsub[I, C]) Publish(
	channel C,
	data ...any,
) error {
	ps.mu.Lock()

	// check if channel exists and copy handlers
	subs, ok := ps.handlers[channel]
	if !ok {
		ps.mu.Unlock()
		return nil
	}

	// Copy handlers to a slice to release the lock
	handlers := make([]PubSubHandler[I, C], 0, len(subs))
	for _, handler := range subs {
		handlers = append(handlers, handler)
	}
	ps.mu.Unlock()

	// execute handlers asynchronously
	var wg sync.WaitGroup
	errorChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h PubSubHandler[I, C]) {
			defer wg.Done()
			if e := h(channel, data...); e != nil {
				errorChan <- e
			}
		}(handler)
	}

	// wait for all handlers to complete
	wg.Wait()
	close(errorChan)

	// collect errors
	var errors []error
	for e := range errorChan {
		errors = append(errors, e)
	}
	if len(errors) > 0 {
		return newErrPublishFailed(errors)
	}

	return nil
}
