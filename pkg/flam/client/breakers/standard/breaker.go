package standard

import (
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// breaker is the standard [flam.IClientCircuitBreaker], opening after a
// run of consecutive failures, refusing the requests while open, and,
// once its open timeout elapsed, letting its half open requests through as
// trials: their success closes it, while a failed one opens it again.
//
// Every change of state starts a new generation, and the requests report
// their outcome with the generation they were let through under: a
// request let through before a change does not count against the state
// following it, so that a slow failure from before the circuit opened
// cannot open again a circuit that recovered since.
type breaker struct {
	id               string
	clock            flam.IClock
	failureThreshold int
	openTimeout      time.Duration
	halfOpenRequests int

	mu         sync.Mutex
	state      flam.ClientCircuitState
	generation uint64
	failures   int
	openedAt   time.Time
	trials     int
	successes  int
}

// compile time check that *breaker implements IClientCircuitBreaker
var _ flam.IClientCircuitBreaker = (*breaker)(nil)

func (b *breaker) Id() string {
	return b.id
}

// State returns the state of the circuit, half open once the open timeout
// of an open circuit elapsed.
func (b *breaker) State() flam.ClientCircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refresh()
	return b.state
}

// Allow lets every request through while the circuit is closed, none while
// it is open, and the half open requests while it is half open, until
// their outcomes are reported, an ignored one freeing its slot.
func (b *breaker) Allow() (uint64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refresh()
	switch b.state {
	case flam.ClientCircuitOpen:
		return 0, false
	case flam.ClientCircuitHalfOpen:
		if b.trials >= b.halfOpenRequests {
			return 0, false
		}
		b.trials++
	}
	return b.generation, true
}

// Done counts the outcome of a request of the current generation: a
// failure opens a half open circuit, or a closed one reaching the failure
// threshold, while a success resets the failures of a closed circuit, and
// closes a half open one once its half open requests all succeeded.
func (b *breaker) Done(
	generation uint64,
	outcome flam.ClientOutcome,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refresh()
	if generation != b.generation {
		return
	}

	switch b.state {
	case flam.ClientCircuitClosed:
		switch outcome {
		case flam.ClientOutcomeSuccess:
			b.failures = 0
		case flam.ClientOutcomeFailure:
			b.failures++
			if b.failures >= b.failureThreshold {
				b.transition(flam.ClientCircuitOpen)
			}
		}
	case flam.ClientCircuitHalfOpen:
		switch outcome {
		case flam.ClientOutcomeSuccess:
			b.successes++
			if b.successes >= b.halfOpenRequests {
				b.transition(flam.ClientCircuitClosed)
			}
		case flam.ClientOutcomeFailure:
			b.transition(flam.ClientCircuitOpen)
		default:
			b.trials--
		}
	}
}

// refresh makes an open circuit half open once its open timeout elapsed.
func (b *breaker) refresh() {
	if b.state == flam.ClientCircuitOpen && b.clock.Since(b.openedAt) >= b.openTimeout {
		b.transition(flam.ClientCircuitHalfOpen)
	}
}

// transition moves the circuit to the state, starting a new generation.
func (b *breaker) transition(
	state flam.ClientCircuitState,
) {
	b.state = state
	b.generation++
	b.failures = 0
	b.trials = 0
	b.successes = 0
	if state == flam.ClientCircuitOpen {
		b.openedAt = b.clock.Now()
	}
}
