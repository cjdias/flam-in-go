package flam

import (
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

type PubSubHandler func(data ...any) error

type PubSub interface {
	Subscribe(id string, channel string, handler PubSubHandler) error
	Unsubscribe(id string) error
	Publish(channel string, data ...any) error
}

type pubsubReg struct {
	channel string
	handler PubSubHandler
}

type pubsub struct {
	locker   sync.Locker
	handlers map[string]pubsubReg
}

var _ PubSub = (*pubsub)(nil)

func NewPubSub() PubSub {
	return &pubsub{
		locker:   &sync.Mutex{},
		handlers: map[string]pubsubReg{},
	}
}

func (ps *pubsub) Subscribe(
	id string,
	channel string,
	handler PubSubHandler,
) error {
	if doublestar.ValidatePattern(channel) == false {
		return newErrInvalidSubscriptionChannelPattern(channel)
	}

	ps.locker.Lock()
	defer ps.locker.Unlock()

	if _, ok := ps.handlers[id]; ok {
		return newErrDuplicateSubscription(id)
	}

	ps.handlers[id] = pubsubReg{
		channel: channel,
		handler: handler}

	return nil
}

func (ps *pubsub) Unsubscribe(
	id string,
) error {
	ps.locker.Lock()
	defer ps.locker.Unlock()

	if _, ok := ps.handlers[id]; ok {
		delete(ps.handlers, id)

		return nil
	}

	return newErrSubscriptionNotFound(id)
}

func (ps *pubsub) Publish(
	channel string,
	data ...any,
) error {
	ps.locker.Lock()
	defer ps.locker.Unlock()

	for _, reg := range ps.handlers {
		if doublestar.MatchUnvalidated(reg.channel, channel) {
			if e := reg.handler(data...); e != nil {
				return e
			}
		}
	}

	return nil
}
