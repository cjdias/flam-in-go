package tests

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_PulseTrigger(t *testing.T) {
	t.Run("should return the defined delay", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		expectedDelay := 20 * time.Millisecond
		handler := func() error {
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewPulse(expectedDelay, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Equal(t, expectedDelay, got.Delay())

			assert.NoError(t, got.Close())
		}))
	})

	t.Run("should return the correct closed state", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		handler := func() error {
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewPulse(20*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.True(t, got.IsRunning())

			assert.NoError(t, got.Close())
			assert.False(t, got.IsRunning())
		}))
	})

	t.Run("should execute the callback after the delay", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		var wg sync.WaitGroup
		wg.Add(1)

		callbackExecuted := false
		handler := func() error {
			callbackExecuted = true
			wg.Done()
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewPulse(10*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			wg.Wait()
			assert.True(t, callbackExecuted)
			assert.NoError(t, got.Close())
		}))
	})

	t.Run("should not execute the callback if closed before delay", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		callbackExecuted := false
		handler := func() error {
			callbackExecuted = true
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewPulse(20*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.NoError(t, got.Close())

			time.Sleep(30 * time.Millisecond)
			assert.False(t, callbackExecuted)
		}))
	})
}
