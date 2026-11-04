package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// relay is the [flam.IWatchdogProcess] publishing the messages of the
// outbox with a broker.
type relay struct {
	id         string // the process id
	relayId    string
	conn       flam.IDatabaseConnection
	broker     flam.IBroker
	clock      flam.IClock
	logger     flam.ILogger // nil without logger
	poll       time.Duration
	batch      int
	lease      time.Duration
	backoff    time.Duration
	maxBackoff time.Duration
	delete     bool
	retention  time.Duration
	cleanup    time.Duration

	stop chan struct{} // closed by Close

	mu      sync.Mutex
	running bool
	closed  bool

	lastCleanup time.Time // read and written by Run only
}

// compile time check that *relay implements IWatchdogProcess
var _ flam.IWatchdogProcess = (*relay)(nil)

func (r *relay) Id() string {
	return r.id
}

func (r *relay) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// Run publishes the messages of the outbox, a batch after the other,
// polling again once a batch is not full, until the context is done or
// the relay is closed, the batch under way being completed. A nil context
// never ends. It returns nil, the failures being logged, fails with
// ErrRunning when the relay runs already, and returns at once once the
// relay is closed.
func (r *relay) Run(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	switch {
	case r.closed:
		r.mu.Unlock()
		return nil
	case r.running:
		r.mu.Unlock()
		return newErrRelay(ErrRunning, relayRunMessage, r.relayId, nil)
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-r.stop:
			cancel()
		case <-runCtx.Done():
		}
	}()
	base := context.WithoutCancel(ctx)

	for runCtx.Err() == nil {
		r.clean(base)
		if !r.publishBatch(base) {
			r.wait(runCtx)
		}
	}
	return nil
}

// Close stops the relay, making a Run under way return once its batch is
// completed, and the later ones return at once.
func (r *relay) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		close(r.stop)
	}
	return nil
}

// now returns the time of the clock, in UTC, truncated to the microsecond
// as the databases store it.
func (r *relay) now() time.Time {
	return r.clock.Now().UTC().Truncate(time.Microsecond)
}

// wait waits for the poll delay, or until the run context is done.
func (r *relay) wait(
	runCtx context.Context,
) {
	timer := r.clock.NewTimer(r.poll)
	defer timer.Stop()
	select {
	case <-timer.C():
	case <-runCtx.Done():
	}
}

// clean deletes the sent messages past their retention, once per cleanup
// delay, logging the failure.
func (r *relay) clean(
	ctx context.Context,
) {
	now := r.now()
	if !r.lastCleanup.IsZero() && now.Sub(r.lastCleanup) < r.cleanup {
		return
	}
	r.lastCleanup = now
	err := r.conn.WithContext(ctx).
		Where("sent_at IS NOT NULL AND sent_at < ?", now.Add(-r.retention)).
		Delete(&Row{}).Error
	if err != nil {
		r.log(flam.LogError, newErrRelay(ErrCleanup, relayCleanMessage, r.relayId, err))
	}
}

// publishBatch claims a batch of messages, then publishes them in order,
// returning whether the batch was full, so that more messages may be
// pending. Once a message of a topic fails, or cannot be claimed, the
// later messages of the topic are left for later, keeping their order.
func (r *relay) publishBatch(
	ctx context.Context,
) bool {
	now := r.now()
	db := r.conn.WithContext(ctx)
	var rows []Row
	err := db.
		Where("sent_at IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (locked_until IS NULL OR locked_until <= ?)", now, now).
		Where("NOT EXISTS (SELECT 1 FROM "+Table+" e WHERE e.topic = "+Table+".topic AND e.id < "+Table+".id AND e.sent_at IS NULL AND (e.locked_until > ? OR e.next_attempt_at > ?))", now, now).
		Order("id").
		Limit(r.batch).
		Find(&rows).Error
	if err != nil {
		r.log(flam.LogError, newErrRelay(ErrClaim, relayClaimMessage, r.relayId, err))
		return false
	}

	skipped := map[string]bool{}
	claimed := make([]Row, 0, len(rows))
	until := now.Add(r.lease)
	for _, row := range rows {
		if skipped[row.Topic] {
			continue
		}
		token := newToken()
		res := db.Model(&Row{}).
			Where("id = ? AND token = ? AND sent_at IS NULL", row.ID, row.Token).
			Updates(map[string]any{"token": token, "locked_until": until})
		if res.Error != nil {
			r.log(flam.LogError, newErrMessage(ErrClaim, relayClaimMessage, r.relayId, &row, res.Error))
		}
		if res.Error != nil || res.RowsAffected == 0 {
			skipped[row.Topic] = true
			continue
		}
		row.Token, row.LockedUntil = token, &until
		claimed = append(claimed, row)
	}

	failed := map[string]bool{}
	for _, row := range claimed {
		if failed[row.Topic] {
			r.record(ctx, &row, map[string]any{"locked_until": nil})
			continue
		}
		if err := r.publish(ctx, &row); err != nil {
			failed[row.Topic] = true
			r.log(flam.LogWarning, err)
			r.record(ctx, &row, map[string]any{
				"attempts":        row.Attempts + 1,
				"next_attempt_at": r.now().Add(r.delay(row.Attempts + 1)),
				"locked_until":    nil,
				"last_error":      err.Error(),
			})
			continue
		}
		if r.delete {
			r.record(ctx, &row, nil)
		} else {
			r.record(ctx, &row, map[string]any{"sent_at": r.now(), "locked_until": nil})
		}
	}
	return len(rows) == r.batch
}

// publish publishes the message with the broker, returning the error
// failing it, if any.
func (r *relay) publish(
	ctx context.Context,
	row *Row,
) *flam.Error {
	var headers map[string]string
	if row.Headers != "" {
		if err := json.Unmarshal([]byte(row.Headers), &headers); err != nil {
			return newErrMessage(ErrDecode, relayPublishMessage, r.relayId, row, err)
		}
	}
	if err := r.broker.Publish(ctx, row.Topic, row.Payload, headers); err != nil {
		return newErrMessage(ErrPublish, relayPublishMessage, r.relayId, row, err).
			With(ErrorFieldBroker, r.broker.Id())
	}
	return nil
}

// record updates the message with the values, or deletes it when nil, as
// long as the relay holds its claim, logging the failure.
func (r *relay) record(
	ctx context.Context,
	row *Row,
	values map[string]any,
) {
	db := r.conn.WithContext(ctx).Model(&Row{}).Where("id = ? AND token = ?", row.ID, row.Token)
	var res *gorm.DB
	if values == nil {
		res = db.Delete(&Row{})
	} else {
		res = db.Updates(values)
	}
	switch {
	case res.Error != nil:
		r.log(flam.LogError, newErrMessage(ErrRecord, relayRecordMessage, r.relayId, row, res.Error))
	case res.RowsAffected == 0:
		r.log(flam.LogWarning, newErrMessage(ErrLeaseLost, relayRecordMessage, r.relayId, row, nil))
	}
}

// delay returns the backoff delay of the retry following the attempt:
// the backoff doubled at each attempt after the first, up to the maximum.
func (r *relay) delay(
	attempt int,
) time.Duration {
	d := r.backoff
	for i := 1; i < attempt && d < r.maxBackoff; i++ {
		if d > r.maxBackoff/2 {
			d = r.maxBackoff
		} else {
			d *= 2
		}
	}
	return d
}

// log logs the error at the level on LogChannel, with its context, when
// the relay has a logger.
func (r *relay) log(
	level flam.LogLevel,
	err *flam.Error,
) {
	if r.logger != nil {
		r.logger.Signal(level, LogChannel, err.Error(), container.Bag(err.Context()))
	}
}

// newToken returns a new random claim token.
func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails
	return hex.EncodeToString(b)
}

// Messages of the errors of the relay.
const (
	relayRunMessage     = "unable to run outbox relay"
	relayClaimMessage   = "unable to claim outbox messages"
	relayPublishMessage = "unable to publish outbox message"
	relayRecordMessage  = "unable to record outbox message outcome"
	relayCleanMessage   = "unable to clean up outbox messages"
)
