package funit

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// source is the [flam.IConfigSource] of the values of a configuration
// file or of WithConfigValues, added to the configuration of the
// application for the time of a run.
type source struct {
	id       string
	priority int
	values   flam.IBag
}

// compile time check that *source implements IConfigSource
var _ flam.IConfigSource = (*source)(nil)

func (s *source) Id() string {
	return s.id
}

func (s *source) Priority() int {
	return s.priority
}

// Bag returns a copy of the values of the source.
func (s *source) Bag() flam.IBag {
	return s.values.Clone()
}

func (s *source) Close() error {
	return nil
}
