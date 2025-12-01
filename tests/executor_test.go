package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go"
)

func Test_Executor_NewExecutor(t *testing.T) {
	executor := flam.NewExecutor()
	assert.NotNil(t, executor)
}

func Test_Executor_Run(t *testing.T) {
	t.Run("should return error on a nil container reference", func(t *testing.T) {
		executor := flam.NewExecutor()
		assert.ErrorIs(t, executor.Run(nil), flam.ErrNilReference)
	})

	t.Run("should run without error on an empty queue", func(t *testing.T) {
		executor := flam.NewExecutor()
		assert.NoError(t, executor.Run(dig.New()))
	})

	t.Run("should return an error if executing a non-function callback fails", func(t *testing.T) {
		executor := flam.NewExecutor().Queue(struct{}{})

		assert.Error(t, executor.Run(dig.New()))
	})

	t.Run("should return an error if executing a callback without a requested dependency", func(t *testing.T) {
		type Dep struct{}
		callback := func(d *Dep) {
			assert.NotNil(t, d)
		}

		container := dig.New()
		executor := flam.NewExecutor().Queue(callback)

		assert.Error(t, executor.Run(container))
	})

	t.Run("should successfully execute a callback", func(t *testing.T) {
		type Dep struct{}
		constructor := func() *Dep { return &Dep{} }
		callback := func(d *Dep) {
			assert.NotNil(t, d)
		}

		container := dig.New()
		registerer := flam.NewRegisterer().Queue(constructor)
		executor := flam.NewExecutor().Queue(callback)

		require.NoError(t, registerer.Run(container))
		assert.NoError(t, executor.Run(container))
	})
}
