package funit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewRunner returns the runner of a functional test of the application,
// reporting its failures to the test. It fails with ErrNilApp or
// ErrNilTest when one of them is nil.
func NewRunner(
	app flam.IApp,
	t testing.TB,
) (*Runner, error) {
	switch {
	case isNil(app):
		return nil, newErr(ErrNilApp, runnerNewMessage, nil)
	case isNil(t):
		return nil, newErr(ErrNilTest, runnerNewMessage, nil)
	}
	return &Runner{
		id:               newId(),
		t:                t,
		app:              app,
		logger:           &logger{},
		startTimeout:     DefaultStartTimeout,
		stopTimeout:      DefaultStopTimeout,
		publishedTimeout: DefaultPublishedTimeout,
	}, nil
}

// Runner takes an application through the stages of a functional test,
// described by the package documentation, configured by its With methods,
// then run by Run. Its methods are meant for the goroutine of the test.
type Runner struct {
	id               string
	t                testing.TB
	app              flam.IApp
	logger           Logger
	startTimeout     time.Duration
	stopTimeout      time.Duration
	publishedTimeout time.Duration
	err              error // the first error of the builders

	configs   []configReg
	processes []string
	arranges  []stage
	act       *stage
	asserts   []stage
	teardowns []stage
	topics    []string

	ran       bool
	added     []*source    // the configuration sources added
	activated []string     // the processes activated, inactive before
	kennel    flam.IKennel // the kennel of the application, if any
	done      chan error   // the end of the run of the application
	pubsub    flam.IPubSub[string, string]
	conn      flam.IDatabaseConnection
	tx        *gorm.DB
	redis     flam.IRedisConnection
	mu        sync.Mutex
	records   []Published
}

// Published is a message published on a topic the runner subscribed to,
// with WithArrangeSignals.
type Published struct {
	// Topic is the topic the message was published on.
	Topic string

	// Message is the message.
	Message string
}

// WithLogger sets the logger reporting the progress of the runner, a nil
// one reporting nothing, as the runner does unless set.
func (r *Runner) WithLogger(
	l Logger,
) *Runner {
	if isNil(l) {
		l = &logger{}
	}
	r.logger = l
	return r
}

// WithStartTimeout sets the time the runner waits for the processes of
// the test to run, [DefaultStartTimeout] unless set. A timeout of 0 or
// less leaves it unchanged.
func (r *Runner) WithStartTimeout(
	timeout time.Duration,
) *Runner {
	if timeout > 0 {
		r.startTimeout = timeout
	}
	return r
}

// WithStopTimeout sets the time the runner waits for the run of the
// application to end once its processes are closed, [DefaultStopTimeout]
// unless set. A timeout of 0 or less leaves it unchanged.
func (r *Runner) WithStopTimeout(
	timeout time.Duration,
) *Runner {
	if timeout > 0 {
		r.stopTimeout = timeout
	}
	return r
}

// WithPublishedTimeout sets the time the assertion of WithAssertPublished
// waits for the messages, [DefaultPublishedTimeout] unless set. A timeout
// of 0 or less leaves it unchanged.
func (r *Runner) WithPublishedTimeout(
	timeout time.Duration,
) *Runner {
	if timeout > 0 {
		r.publishedTimeout = timeout
	}
	return r
}

// WithConfig adds the values of the configuration file, read by the
// configuration parser of the id, [DefaultConfigParser] unless given or
// empty, to
// the configuration of the application, with the priority, for the time
// of the run.
func (r *Runner) WithConfig(
	file string,
	priority int,
	parser ...string,
) *Runner {
	id := DefaultConfigParser
	if len(parser) > 0 && parser[0] != "" {
		id = parser[0]
	}
	r.configs = append(r.configs, configReg{file: file, priority: priority, parser: id})
	return r
}

// WithConfigValues adds the values, such as a container.Bag, to the
// configuration of the application, with the priority, for the time of
// the run.
func (r *Runner) WithConfigValues(
	values flam.IBag,
	priority int,
) *Runner {
	r.configs = append(r.configs, configReg{values: values, priority: priority})
	return r
}

// WithProcess activates the watchdog process of the id, typically
// configured inactive, for the run, which waits for it to run before the
// act, failing when the run of the application ends before. A process
// inactive before is deactivated once the run ends. It needs the watchdog
// bundle.
func (r *Runner) WithProcess(
	id string,
) *Runner {
	r.processes = append(r.processes, id)
	return r
}

// WithArrange adds the arrange function of the id, invoked through the
// container of the application once it booted, failing the run when it
// returns an error as its last result.
func (r *Runner) WithArrange(
	id string,
	function any,
) *Runner {
	r.arranges = append(r.arranges, r.stage("WithArrange", id, function))
	return r
}

// WithArrangePubSub takes the in-process pub/sub of the application, of
// string topics and messages, returned then by PubSub.
func (r *Runner) WithArrangePubSub() *Runner {
	return r.WithArrange("pubsub", func(pubsub flam.IPubSub[string, string]) {
		r.pubsub = pubsub
	})
}

// WithArrangeSignals takes the in-process pub/sub of the application, as
// WithArrangePubSub does, and subscribes to the topics, recording the
// messages published on them, returned then by Published and checked by
// WithAssertPublished. A topic is subscribed to once, however many times
// given. The subscriptions are cancelled once the run ends.
func (r *Runner) WithArrangeSignals(
	topics ...string,
) *Runner {
	return r.WithArrange("signals", func(pubsub flam.IPubSub[string, string]) error {
		r.pubsub = pubsub
		for _, topic := range topics {
			if slices.Contains(r.topics, topic) {
				continue
			}
			if err := pubsub.Subscribe(r.subscription(topic), topic, r.record, nil); err != nil {
				return err
			}
			r.topics = append(r.topics, topic)
		}
		return nil
	})
}

// WithArrangeDatabase begins a transaction on the database connection of
// the id, returned then by Tx, and by Connection as a connection, rolled
// back once the run ends, so that the test leaves the database as it found
// it.
func (r *Runner) WithArrangeDatabase(
	id string,
) *Runner {
	return r.WithArrange("database", func(connections flam.IDatabaseConnectionFactory) error {
		conn, err := connections.Get(id)
		if err != nil {
			return err
		}
		tx := conn.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		r.tx = tx
		r.conn = conn.WithConnection(tx)
		return nil
	})
}

// WithArrangeRedis takes the redis connection of the id, returned then by
// Redis.
func (r *Runner) WithArrangeRedis(
	id string,
) *Runner {
	return r.WithArrange("redis", func(connections flam.IRedisConnectionFactory) error {
		conn, err := connections.Get(id)
		if err != nil {
			return err
		}
		r.redis = conn
		return nil
	})
}

// WithAct sets the act function, invoked through the container of the
// application, failing the run when it returns an error as its last
// result. A run without act only arranges and asserts.
func (r *Runner) WithAct(
	function any,
) *Runner {
	act := r.stage("WithAct", "act", function)
	r.act = &act
	return r
}

// WithAssert adds the assertion of the id, invoked through the container
// of the application after the act, which reports its mismatches to the
// test, or returns an error as its last result failing the run.
func (r *Runner) WithAssert(
	id string,
	function any,
) *Runner {
	r.asserts = append(r.asserts, r.stage("WithAssert", id, function))
	return r
}

// WithTeardown adds the teardown function of the id, invoked through the
// container of the application once the run of the application ended,
// whatever the stages before it did, all of them running even when one
// fails.
func (r *Runner) WithTeardown(
	id string,
	function any,
) *Runner {
	r.teardowns = append(r.teardowns, r.stage("WithTeardown", id, function))
	return r
}

// PubSub returns the pub/sub taken by WithArrangePubSub or
// WithArrangeSignals, or nil before.
func (r *Runner) PubSub() flam.IPubSub[string, string] {
	return r.pubsub
}

// Tx returns the transaction begun by WithArrangeDatabase, or nil before.
func (r *Runner) Tx() *gorm.DB {
	return r.tx
}

// Connection returns the database connection of the transaction begun by
// WithArrangeDatabase, which does not own its pool, or nil before.
func (r *Runner) Connection() flam.IDatabaseConnection {
	return r.conn
}

// Redis returns the redis connection taken by WithArrangeRedis, or nil
// before.
func (r *Runner) Redis() flam.IRedisConnection {
	return r.redis
}

// Published returns the messages recorded on the topics of
// WithArrangeSignals, in the order they were delivered.
func (r *Runner) Published() []Published {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Published{}, r.records...)
}

// recorded returns the number of the messages recorded.
func (r *Runner) recorded() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

// Message of the errors returned by NewRunner.
const runnerNewMessage = "unable to create funit runner"

// configReg is a configuration file, or values, added by the runner.
type configReg struct {
	file     string
	parser   string
	values   flam.IBag
	priority int
}

// stage is a stage function of the runner.
type stage struct {
	id       string
	function any
}

// stage returns the stage of the function, recording the first error of
// the builders, returned by Run, when the id is empty or the function is
// not one.
func (r *Runner) stage(
	builder string,
	id string,
	function any,
) stage {
	if r.err == nil {
		switch {
		case id == "":
			r.err = newErr(ErrInvalidStage, builderMessage, fmt.Errorf("%s: empty id", builder)).
				With(ErrorFieldStage, builder)
		case function == nil || reflect.TypeOf(function).Kind() != reflect.Func || reflect.ValueOf(function).IsNil():
			r.err = newErr(ErrInvalidStage, builderMessage, fmt.Errorf("%s: %T is not a function", builder, function)).
				With(ErrorFieldStage, id)
		}
	}
	return stage{id: id, function: function}
}

// builderMessage is the message of the errors of the builders.
const builderMessage = "invalid funit runner"

// subscription returns the id of the subscription of the runner to the
// topic.
func (r *Runner) subscription(
	topic string,
) string {
	return r.id + "/" + topic
}

// record records the message published on the topic.
func (r *Runner) record(
	_ context.Context,
	topic string,
	message string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, Published{Topic: topic, Message: message})
	return nil
}

// isNil reports whether v is nil, or an interface holding a nil pointer.
func isNil(
	v any,
) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// newId returns a random id, telling the subscriptions of a runner apart
// from those of other runners on the same pub/sub.
func newId() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "funit-" + hex.EncodeToString(b)
}
