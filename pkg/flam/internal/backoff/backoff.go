// Package backoff holds the capped doubling backoff shared by the flam
// packages retrying a work after a failure, such as the jobs, the outbox
// and the database retries.
package backoff

import (
	"time"
)

// Delay returns the delay before the retry following the attempt, from 1:
// the base delay for the first attempt, doubled at each attempt after it,
// up to the maximum, without overflowing however many the attempts.
func Delay(
	base time.Duration,
	maximum time.Duration,
	attempt int,
) time.Duration {
	d := base
	for i := 1; i < attempt && d < maximum; i++ {
		if d > maximum/2 {
			d = maximum
		} else {
			d *= 2
		}
	}
	return min(d, maximum)
}
