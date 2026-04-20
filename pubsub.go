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

	// Check if id already exists in any channel
	for _, channelMap := range ps.handlers {
		if _, ok := channelMap[id]; ok {
			return newErrDuplicateSubscription(id, channel)
		}
	}

	ps.ensureChannelMap(channel)
	ps.handlers[channel][id] = handler

	return nil
}

func (ps *pubsub[I, C]) Unsubscribe(
	id I,
	channel C,
) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if subs, ok := ps.handlers[channel]; ok {
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
	defer ps.mu.Unlock()

	subs, ok := ps.handlers[channel]
	if !ok {
		return nil
	}

	var errors []error
	for _, handler := range subs {
		if e := handler(channel, data...); e != nil {
			errors = append(errors, e)
		}
	}

	if len(errors) > 0 {
		return newErrPublishFailed(errors)
	}

	return nil
}

func (ps *pubsub[I, C]) ensureChannelMap(
	channel C,
) {
	if _, ok := ps.handlers[channel]; !ok {
		ps.handlers[channel] = map[I]PubSubHandler[I, C]{}
	}
}
