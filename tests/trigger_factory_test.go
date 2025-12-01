package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_TriggerFactory_NewPulse(t *testing.T) {
	t.Run("should return an error when callback is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewPulse(1*time.Second, nil)
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrNilReference)
		}))
	})

	t.Run("should create a new pulse trigger", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		handler := func() error {
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewPulse(1*time.Second, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.NoError(t, got.Close())
		}))
	})
}

func Test_TriggerFactory_NewRecurring(t *testing.T) {
	t.Run("should return an error when callback is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewRecurring(1*time.Second, nil)
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrNilReference)
		}))
	})

	t.Run("should create a new recurring trigger", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		handler := func() error {
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewRecurring(1*time.Second, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.NoError(t, got.Close())
		}))
	})
}
