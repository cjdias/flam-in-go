package flam

import (
	"go.uber.org/dig"
)

type Executor interface {
	Queue(callback any) Executor
	Run(container *dig.Container) error
}

type executorEntry struct {
	callback any
}

type executor struct {
	entries []executorEntry
}

var _ Executor = (*executor)(nil)

func NewExecutor() Executor {
	return &executor{
		entries: make([]executorEntry, 0)}
}

func (executor *executor) Queue(
	callback any,
) Executor {
	executor.entries = append(
		executor.entries,
		executorEntry{
			callback: callback})

	return executor
}

func (executor *executor) Run(
	container *dig.Container,
) error {
	if container == nil {
		return newErrNilReference("container")
	}

	for _, entry := range executor.entries {
		if e := container.Invoke(entry.callback); e != nil {
			return e
		}
	}

	return nil
}
