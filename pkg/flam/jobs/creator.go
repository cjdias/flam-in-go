package jobs

import (
	"errors"
	"slices"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the connection and broker factories being optional so that
// the creator fails with an error telling which bundle is missing, and the
// logger optional so that the workers log nothing without one. The group
// tag repeats HandlerGroup, as a struct tag cannot use a constant.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Clock       flam.IClock
	Connections flam.IDatabaseConnectionFactory `optional:"true"`
	Brokers     flam.IBrokerFactory             `optional:"true"`
	Logger      flam.ILogger                    `optional:"true"`
	Handlers    []flam.IJobHandler              `group:"flam.jobs.handlers"`
}

// newCreator returns the [flam.IJobQueueCreator] of the queues, reading
// the defaults of the missing fields from the configuration. It is the
// provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IJobQueueCreator {
	return &creator{args: args}
}

// creator is the [flam.IJobQueueCreator] of the configurations with no
// driver or the [Driver] driver.
type creator struct {
	args creatorArgs
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	driver := bag.Get(factory.ConfigFieldDriver)
	return driver == nil || driver == Driver
}

// Create returns the queue of the fields, failing at the first invalid
// one, over the database connection of its connection field, taken from
// the connection factory, which keeps owning it, after creating or
// updating its table unless its migrate field is false.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IJobQueue, error) {
	id := bag.String(factory.ConfigFieldId)
	r := &reader{config: c.args.Config, bag: bag}

	connection := r.string(ConfigFieldConnection, DefaultConnectionConfigPath, "", true)
	q := &queue{
		id:          id,
		clock:       c.args.Clock,
		topic:       r.string(ConfigFieldTopic, "", topicPrefix+id, true),
		maxAttempts: r.int(ConfigFieldMaxAttempts, DefaultMaxAttemptsConfigPath, defaultMaxAttempts, 1),
		logger:      c.args.Logger,
	}
	w := &worker{
		id:          r.string(ConfigFieldProcess, "", processPrefix+id, true),
		queue:       q,
		poll:        r.duration(ConfigFieldPoll, DefaultPollConfigPath, defaultPoll, true),
		concurrency: r.int(ConfigFieldConcurrency, DefaultConcurrencyConfigPath, defaultConcurrency, 1),
		lease:       r.duration(ConfigFieldLease, DefaultLeaseConfigPath, defaultLease, true),
		backoff:     r.duration(ConfigFieldBackoff, DefaultBackoffConfigPath, defaultBackoff, false),
		maxBackoff:  r.duration(ConfigFieldMaxBackoff, DefaultMaxBackoffConfigPath, defaultMaxBackoff, false),
		retention:   r.duration(ConfigFieldRetention, DefaultRetentionConfigPath, defaultRetention, false),
		cleanup:     r.duration(ConfigFieldCleanup, DefaultCleanupConfigPath, defaultCleanup, true),
		stopTimeout: r.duration(ConfigFieldStopTimeout, DefaultStopTimeoutConfigPath, defaultStopTimeout, false),
		wakeCh:      make(chan struct{}, 1),
		stop:        make(chan struct{}),
	}
	r.check(ConfigFieldMaxBackoff, w.maxBackoff >= w.backoff)
	locking := r.oneOf(ConfigFieldLocking, DefaultLockingConfigPath, defaultLocking, LockingAuto, LockingSkipLocked, LockingNone)
	migrate := r.bool(ConfigFieldMigrate, DefaultMigrateConfigPath, defaultMigrate)
	brokerId := r.string(ConfigFieldBroker, DefaultBrokerConfigPath, "", false)
	if r.invalid != "" {
		return nil, newErrQueue(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, r.invalid)
	}
	q.worker = w

	handlers, err := c.handlers(id)
	if err != nil {
		return nil, err
	}
	w.handlers = handlers

	if q.conn, err = c.connection(connection); err != nil {
		return nil, newErrQueue(ErrConnection, creatorCreateMessage, id, err).
			With(ErrorFieldConnection, connection)
	}
	if brokerId != "" {
		if q.broker, err = c.broker(brokerId); err != nil {
			return nil, newErrQueue(ErrBroker, creatorCreateMessage, id, err).
				With(ErrorFieldBroker, brokerId)
		}
	}
	if migrate {
		if err := q.conn.AutoMigrate(&Row{}); err != nil {
			return nil, newErrQueue(ErrMigrate, creatorCreateMessage, id, err).
				With(ErrorFieldConnection, connection)
		}
	}
	w.skipLocked = locking == LockingSkipLocked ||
		(locking == LockingAuto && slices.Contains(skipLockedDialects, q.conn.Gorm().Name()))
	return q, nil
}

// handlers returns the handlers of the HandlerGroup group by kind,
// failing with ErrNilHandler or ErrDuplicateKind.
func (c *creator) handlers(
	id string,
) (map[string]flam.IJobHandler, error) {
	handlers := make(map[string]flam.IJobHandler, len(c.args.Handlers))
	for _, h := range c.args.Handlers {
		if h == nil {
			return nil, newErrQueue(ErrNilHandler, creatorCreateMessage, id, nil)
		}
		kind := h.Kind()
		if _, ok := handlers[kind]; ok {
			return nil, newErrQueue(ErrDuplicateKind, creatorCreateMessage, id, nil).
				With(ErrorFieldKind, kind)
		}
		handlers[kind] = h
	}
	return handlers, nil
}

// connection returns the database connection of the id.
func (c *creator) connection(
	id string,
) (flam.IDatabaseConnection, error) {
	if c.args.Connections == nil {
		return nil, errNoConnectionFactory
	}
	return c.args.Connections.Get(id)
}

// broker returns the broker of the id.
func (c *creator) broker(
	id string,
) (flam.IBroker, error) {
	if c.args.Brokers == nil {
		return nil, errNoBrokerFactory
	}
	return c.args.Brokers.Get(id)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create job queue"

// skipLockedDialects are the names of the gorm dialects LockingAuto
// claims with LockingSkipLocked on.
var skipLockedDialects = []string{"postgres", "mysql"}

// Causes of the ErrConnection and ErrBroker errors when the container
// provides no factory.
var (
	errNoConnectionFactory = errors.New("no database connection factory, register the database bundle")
	errNoBrokerFactory     = errors.New("no broker factory, register the broker bundle")
)
