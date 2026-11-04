package jobs

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// job is the [flam.IJob] of a row claimed by a worker.
type job struct {
	row Row
}

// compile time check that *job implements IJob
var _ flam.IJob = (*job)(nil)

func (j *job) Id() string {
	return strconv.FormatUint(j.row.ID, 10)
}

func (j *job) Queue() string {
	return j.row.Queue
}

func (j *job) Kind() string {
	return j.row.Kind
}

func (j *job) Key() string {
	if j.row.DedupKey == nil {
		return ""
	}
	return *j.row.DedupKey
}

func (j *job) Payload() []byte {
	return j.row.Payload
}

func (j *job) Decode(
	target any,
) error {
	return json.Unmarshal(j.row.Payload, target)
}

func (j *job) Attempt() int {
	return j.row.Attempt
}

func (j *job) MaxAttempts() int {
	return j.row.MaxAttempts
}

func (j *job) EnqueuedAt() time.Time {
	return j.row.EnqueuedAt
}
