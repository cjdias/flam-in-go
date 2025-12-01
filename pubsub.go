package flam

import (
	"sync"
)

type PubSubID comparable

type PubSubChannel comparable

type PubSubHandler[I PubSubID, C PubSubChannel] func(channel C, data ...any) error

type PubSub[I PubSubID, C PubSubChannel] interface {
	Subscribe(id I, channel C, handler PubSubHandler[I, C]) error
	Unsubscribe(id I, channel C) error
	Publish(channel C, data ...any) error
}

type pubsub[I PubSubID, C PubSubChannel] struct {
	locker   sync.Locker
	handlers map[C]map[I]PubSubHandler[I, C]
}

var _ PubSub[string, string] = (*pubsub[string, string])(nil)

func NewPubSub[I PubSubID, C PubSubChannel]() PubSub[I, C] {
	return &pubsub[I, C]{
		locker:   &sync.Mutex{},
		handlers: map[C]map[I]PubSubHandler[I, C]{},
	}
}

func (ps *pubsub[I, C]) Subscribe(
	id I,
	channel C,
	handler PubSubHandler[I, C],
) error {
	ps.locker.Lock()
	defer ps.locker.Unlock()

	if _, ok := ps.handlers[channel]; !ok {
		ps.handlers[channel] = map[I]PubSubHandler[I, C]{}
	}

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
	ps.locker.Lock()
	defer ps.locker.Unlock()

	if subs, ok := ps.handlers[channel]; ok {
		if _, ok = ps.handlers[channel][id]; ok {
			delete(subs, id)

			return nil
		}
	}

	return newErrSubscriptionNotFound(id, channel)
}

func (ps *pubsub[I, C]) Publish(
	channel C,
	data ...any,
) error {
	ps.locker.Lock()
	defer ps.locker.Unlock()

	if subs, ok := ps.handlers[channel]; ok {
		for _, handler := range subs {
			if e := handler(channel, data...); e != nil {
				return e
			}
		}
	}

	return nil
}
