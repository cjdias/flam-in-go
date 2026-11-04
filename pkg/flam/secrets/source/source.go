package source

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// source is the secrets [flam.IConfigReloadableSource]. Its configuration
// does not change after its creation; its values change with Reload, while
// mu guards them.
type source struct {
	id       string
	priority int
	store    flam.ISecretStore // nil without values
	values   map[string]string // the keys of the secrets, by path

	reloadMu sync.Mutex // serializes the reloads

	mu  sync.Mutex
	bag flam.IBag
}

// staticSource exposes the methods of [flam.IConfigSource] of the source it
// embeds, and those only, so that a source whose reload is deactivated is
// never reloaded.
type staticSource struct {
	flam.IConfigSource
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

func (s *source) Id() string {
	return s.id
}

// Close closes nothing: the store belongs to the store factory.
func (s *source) Close() error {
	return nil
}

// Priority returns the priority of the source.
func (s *source) Priority() int {
	return s.priority
}

// Bag returns a copy of the secrets of the source, by path.
func (s *source) Bag() flam.IBag {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.bag.Clone()
}

// Reload reads the secrets again. The reloads are serialized, so that a
// slower one never overrides the values of a later one.
func (s *source) Reload() (bool, error) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	if len(s.values) == 0 {
		return false, nil
	}
	bag, err := s.load(sourceReloadMessage)
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if value.Equal(s.bag, bag) {
		return false, nil
	}
	s.bag = bag
	return true, nil
}

// sourceReloadMessage is the message of the errors returned by Reload.
const sourceReloadMessage = "unable to reload secrets config source"

// load returns a new bag of the secrets of the values, read from the store,
// or the error of the first secret that cannot be read.
func (s *source) load(
	message string,
) (flam.IBag, error) {
	bag := container.Bag{}
	for _, path := range slices.Sorted(maps.Keys(s.values)) {
		key := s.values[path]
		secret, err := s.store.Get(context.Background(), key)
		if err != nil {
			return nil, newErrSecret(message, s.id, path, key, err)
		}
		// the paths were checked by the creator: Set cannot fail
		_ = bag.Set(path, secret)
	}
	return bag, nil
}
