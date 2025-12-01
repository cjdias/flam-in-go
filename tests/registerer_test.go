package tests

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go"
)

func Test_Registerer_NewRegisterer(t *testing.T) {
	registerer := flam.NewRegisterer()
	assert.NotNil(t, registerer)
}

func Test_Registerer_Run(t *testing.T) {
	t.Run("should return error on a nil container reference", func(t *testing.T) {
		registerer := flam.NewRegisterer()
		assert.ErrorIs(t, registerer.Run(nil), flam.ErrNilReference)
	})

	t.Run("should run without error on an empty queue", func(t *testing.T) {
		registerer := flam.NewRegisterer()
		assert.NoError(t, registerer.Run(dig.New()))
	})

	t.Run("should return an error if providing a non-function constructor fails", func(t *testing.T) {
		registerer := flam.NewRegisterer().Queue(struct{}{})

		assert.Error(t, registerer.Run(dig.New()))
	})

	t.Run("should successfully provide a constructor to the container", func(t *testing.T) {
		type Dep struct{}
		constructor := func() *Dep {
			return &Dep{}
		}

		container := dig.New()
		registerer := flam.NewRegisterer().Queue(constructor)

		assert.NoError(t, registerer.Run(container))
		assert.NoError(t, container.Invoke(func(d *Dep) {
			assert.NotNil(t, d)
		}))
	})

	t.Run("should register a fallible constructor without immediate error", func(t *testing.T) {
		type Dep struct{}
		expectedErr := errors.New("constructor error")
		constructor := func() (*Dep, error) {
			return nil, expectedErr
		}

		container := dig.New()
		registerer := flam.NewRegisterer().Queue(constructor)

		assert.NoError(t, registerer.Run(container))
		assert.ErrorIs(t, container.Invoke(func(d *Dep) {}), expectedErr)
	})

	t.Run("should stop provision execution on first error", func(t *testing.T) {
		type Dep struct{}
		constructor1 := struct{}{}
		constructor2 := func() *Dep { return &Dep{} }

		container := dig.New()
		registerer := flam.NewRegisterer().Queue(constructor1).Queue(constructor2)

		assert.Error(t, registerer.Run(container))
		assert.Error(t, container.Invoke(func(d *Dep) {}))
	})
}
