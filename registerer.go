package flam

import (
	"go.uber.org/dig"
)

type Registerer interface {
	Queue(constructor any, opts ...dig.ProvideOption) Registerer
	Run(container *dig.Container) error
}

type registererEntry struct {
	constructor any
	opts        []dig.ProvideOption
}

type registerer struct {
	entries []registererEntry
}

var _ Registerer = (*registerer)(nil)

func NewRegisterer() Registerer {
	return &registerer{
		entries: make([]registererEntry, 0)}
}

func (registerer *registerer) Queue(
	constructor any,
	opts ...dig.ProvideOption,
) Registerer {
	registerer.entries = append(
		registerer.entries,
		registererEntry{
			constructor: constructor,
			opts:        opts})

	return registerer
}

func (registerer *registerer) Run(
	container *dig.Container,
) error {
	if container == nil {
		return newErrNilReference("container")
	}

	for _, entry := range registerer.entries {
		if e := container.Provide(entry.constructor, entry.opts...); e != nil {
			return e
		}
	}

	return nil
}
