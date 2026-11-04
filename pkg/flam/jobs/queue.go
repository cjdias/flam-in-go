package jobs

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm/clause"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// queue is the [flam.IJobQueue] created by the creator of the bundle.
type queue struct {
	id          string
	conn        flam.IDatabaseConnection
	clock       flam.IClock
	broker      flam.IBroker // nil without wakeup notifications
	topic       string
	maxAttempts int
	logger      flam.ILogger // nil without logger
	worker      *worker

	mu     sync.Mutex
	closed bool
}

// compile time check that *queue implements IJobQueue
var _ flam.IJobQueue = (*queue)(nil)

func (q *queue) Id() string {
	return q.id
}

// Close closes the queue and stops its worker. It returns nil, and
// calling it again is a no-op.
func (q *queue) Close() error {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	return q.worker.Close()
}

func (q *queue) Enqueue(
	ctx context.Context,
	kind string,
	payload any,
	opts ...flam.JobOption,
) (string, error) {
	options := flam.JobOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	var sentinel error
	switch {
	case closed:
		sentinel = ErrClosed
	case kind == "":
		sentinel = ErrInvalidKind
	case options.MaxAttempts < 0:
		sentinel = ErrInvalidOption
	}
	if sentinel != nil {
		return "", newErrQueue(sentinel, queueEnqueueMessage, q.id, nil).
			With(ErrorFieldKind, kind)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", newErrQueue(ErrEncode, queueEnqueueMessage, q.id, err).
			With(ErrorFieldKind, kind)
	}

	id, err := q.insert(ctx, kind, encoded, options)
	if err != nil {
		return "", newErrQueue(ErrEnqueue, queueEnqueueMessage, q.id, err).
			With(ErrorFieldKind, kind)
	}
	if options.Tx == nil {
		q.Notify(ctx)
	}
	return id, nil
}

func (q *queue) Notify(
	ctx context.Context,
) {
	q.worker.wake()
	if q.broker == nil {
		return
	}
	if err := q.broker.Publish(ctx, q.topic, []byte(q.id), nil); err != nil {
		q.log(flam.LogWarning, newErrQueue(ErrWakeup, queueNotifyMessage, q.id, err).
			With(ErrorFieldBroker, q.broker.Id()))
	}
}

func (q *queue) Worker() flam.IWatchdogProcess {
	return q.worker
}

// now returns the time of the clock, in UTC, truncated to the microsecond
// as the databases store it.
func (q *queue) now() time.Time {
	return q.clock.Now().UTC().Truncate(time.Microsecond)
}

// insert stores the job, or finds the one of its deduplication key, and
// returns its id.
func (q *queue) insert(
	ctx context.Context,
	kind string,
	payload []byte,
	options flam.JobOptions,
) (string, error) {
	now := q.now()
	row := Row{
		Queue:       q.id,
		Kind:        kind,
		Payload:     payload,
		State:       StatePending,
		MaxAttempts: q.maxAttempts,
		RunAt:       now.Add(options.Delay),
		EnqueuedAt:  now,
	}
	if !options.RunAt.IsZero() {
		row.RunAt = options.RunAt.UTC().Truncate(time.Microsecond)
	}
	if options.MaxAttempts > 0 {
		row.MaxAttempts = options.MaxAttempts
	}
	if options.Key != "" {
		row.DedupKey = &options.Key
	}

	db := options.Tx
	if db == nil {
		db = q.conn.Gorm()
	}
	db = db.WithContext(ctx)
	res := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		var existing Row
		if err := db.Where("queue = ? AND dedup_key = ?", q.id, options.Key).Take(&existing).Error; err != nil {
			return "", err
		}
		row.ID = existing.ID
	}
	return strconv.FormatUint(row.ID, 10), nil
}

// log logs the error at the level on LogChannel, with its context, when
// the queue has a logger.
func (q *queue) log(
	level flam.LogLevel,
	err *flam.Error,
) {
	if q.logger != nil {
		q.logger.Signal(level, LogChannel, err.Error(), container.Bag(err.Context()))
	}
}

// Messages of the errors of the queue.
const (
	queueEnqueueMessage = "unable to enqueue job"
	queueNotifyMessage  = "unable to notify job queue"
)
