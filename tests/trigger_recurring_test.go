package tests

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_RecurringTrigger(t *testing.T) {
	t.Run("should return the defined delay", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		handler := func() error {
			return nil
		}

		expectedDelay := 20 * time.Millisecond
		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewRecurring(expectedDelay, handler)
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
			got, e := factory.NewRecurring(20*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.True(t, got.IsRunning())

			assert.NoError(t, got.Close())
			assert.False(t, got.IsRunning())
		}))
	})

	t.Run("should execute the callback multiple times", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		var wg sync.WaitGroup
		wg.Add(3)

		callCount := 0
		handler := func() error {
			callCount++
			wg.Done()
			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewRecurring(10*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			wg.Wait()
			assert.GreaterOrEqual(t, callCount, 3)

			assert.NoError(t, got.Close())
		}))
	})

	t.Run("should stop when callback returns an error", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		callCount := 0
		handler := func() error {
			callCount++
			if callCount == 2 {
				return errors.New("stop error")
			}

			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewRecurring(10*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			time.Sleep(50 * time.Millisecond)
			assert.Equal(t, 2, callCount)

			assert.NoError(t, got.Close())
		}))
	})

	t.Run("should not execute the callback if closed", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		callCount := 0
		handler := func() error {
			callCount++

			return nil
		}

		assert.NoError(t, app.Container().Invoke(func(factory flam.TriggerFactory) {
			got, e := factory.NewRecurring(10*time.Millisecond, handler)
			require.NotNil(t, got)
			require.NoError(t, e)

			time.Sleep(15 * time.Millisecond) // Allow it to run at least once
			assert.NoError(t, got.Close())

			firstCount := callCount
			time.Sleep(30 * time.Millisecond)
			assert.Equal(t, firstCount, callCount)
		}))
	})
}
