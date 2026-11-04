package memory

import (
	"context"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// store holds the keys of the locks of a memory locker, each under the
// token of its holder until it expires, and the fence of each key ever
// acquired. A done context fails its methods with its error, as a redis
// connection would.
type store struct {
	clock flam.IClock

	mu      sync.Mutex // guards the fields below
	entries map[string]entry
	fences  map[string]int64
}

// entry is a key held by a store.
type entry struct {
	token   string
	expires time.Time
}

// Acquire holds the key under the token for the ttl, unless held, and
// returns its new fence.
func (s *store) Acquire(
	ctx context.Context,
	key string,
	token string,
	ttl time.Duration,
) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	if e, ok := s.entries[key]; ok && now.Before(e.expires) {
		return 0, false, nil
	}
	s.fences[key]++
	s.entries[key] = entry{token: token, expires: now.Add(ttl)}
	return s.fences[key], true, nil
}

// Refresh holds the key under the token for the ttl from now, when it is
// held under the token.
func (s *store) Refresh(
	ctx context.Context,
	key string,
	token string,
	ttl time.Duration,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	if !s.holds(key, token, now) {
		return false, nil
	}
	s.entries[key] = entry{token: token, expires: now.Add(ttl)}
	return true, nil
}

// Release removes the key, when it is held under the token.
func (s *store) Release(
	ctx context.Context,
	key string,
	token string,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.holds(key, token, s.clock.Now()) {
		return false, nil
	}
	delete(s.entries, key)
	return true, nil
}

// newStore returns an empty store reading the time from the clock.
func newStore(
	clock flam.IClock,
) *store {
	return &store{clock: clock, entries: map[string]entry{}, fences: map[string]int64{}}
}

// holds reports whether the key is held under the token, unexpired at
// now. The lock must be held.
func (s *store) holds(
	key string,
	token string,
	now time.Time,
) bool {
	e, ok := s.entries[key]
	return ok && e.token == token && now.Before(e.expires)
}
