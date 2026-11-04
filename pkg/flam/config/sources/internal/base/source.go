package base

import (
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// New returns the base of a source with the id and priority, holding no
// value until the first Swap.
func New(
	id string,
	priority int,
) *Source {
	return &Source{id: id, priority: priority, bag: container.Bag{}}
}

// Source implements the methods of [flam.IConfigSource] for the drivers
// embedding it, and holds the values of the source, replaced by Swap. Its
// id and priority do not change after its creation. It is safe for
// concurrent use.
type Source struct {
	id       string
	priority int

	mu  sync.Mutex // guards bag
	bag flam.IBag
}

// Id returns the id of the source.
func (s *Source) Id() string {
	return s.id
}

// Close closes nothing: the drivers hold nothing of their own to close.
func (s *Source) Close() error {
	return nil
}

// Priority returns the priority of the source.
func (s *Source) Priority() int {
	return s.priority
}

// Bag returns a copy of the values of the source.
func (s *Source) Bag() flam.IBag {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.bag.Clone()
}

// Swap replaces the values of the source with the new ones, unless they are
// equal as [value.Equal] tells, and reports whether they were replaced.
func (s *Source) Swap(
	values flam.IBag,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if value.Equal(s.bag, values) {
		return false
	}
	s.bag = values
	return true
}
