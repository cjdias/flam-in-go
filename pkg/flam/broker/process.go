package broker

import (
	"context"
	"errors"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewProcess returns the [flam.IWatchdogProcess] of the id, subscribing
// the handler to the topic of the broker while it runs, so that the kennel
// of [github.com/cjdias/flam-in-go/pkg/flam/watchdog] runs a long-lived
// subscription as it runs the servers. Provide it in the
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog.ProcessGroup] group,
// taking the broker from the [flam.IBrokerFactory]:
//
//	app.Provide(func(brokers flam.IBrokerFactory) (flam.IWatchdogProcess, error) {
//		b, err := brokers.Get("events")
//		if err != nil {
//			return nil, err
//		}
//		return broker.NewProcess("orders", b, "orders.created", handleOrder)
//	}, dig.Group(watchdog.ProcessGroup))
//
// Its Run subscribes, then waits until its context is done, the process
// is closed or the subscription ends, as when its broker closes, and
// closes the subscription, returning the error that ended it, or the one
// of the Subscribe method of the broker. It fails with ErrRunning when the
// process runs already, and returns at once once the process is closed.
// Closing the process does not close the broker, which its factory owns.
//
// NewProcess fails with ErrNilBroker or ErrNilHandler, holding the id and
// the topic.
func NewProcess(
	id string,
	b flam.IBroker,
	topic string,
	handler flam.BrokerHandler,
) (flam.IWatchdogProcess, error) {
	var sentinel error
	switch {
	case b == nil:
		sentinel = ErrNilBroker
	case handler == nil:
		sentinel = ErrNilHandler
	default:
		return &process{id: id, broker: b, topic: topic, handler: handler, stop: make(chan struct{})}, nil
	}
	return nil, newErr(sentinel, processNewMessage, nil).
		With(ErrorFieldID, id).
		With(ErrorFieldTopic, topic)
}

// process is the [flam.IWatchdogProcess] returned by [NewProcess].
type process struct {
	id      string
	broker  flam.IBroker
	topic   string
	handler flam.BrokerHandler

	stop    chan struct{}
	mu      sync.Mutex
	running bool
	closed  bool
}

func (p *process) Id() string {
	return p.id
}

func (p *process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// Run subscribes the handler to the topic until the context is done, the
// process is closed or the subscription ends. A nil context never ends.
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
		return newErr(ErrRunning, processRunMessage, nil).
			With(ErrorFieldID, p.id).
			With(ErrorFieldTopic, p.topic)
	}
	p.running = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	sub, err := p.broker.Subscribe(ctx, p.topic, p.handler)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case <-p.stop:
	case <-sub.Done():
	}
	closeErr := sub.Close()
	return errors.Join(sub.Err(), closeErr)
}

// Close stops the process, making a Run under way close its subscription
// and return, and the later ones return at once.
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
	processNewMessage = "unable to create broker process"
	processRunMessage = "unable to run broker process"
)
