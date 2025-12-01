package tests

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_PubSub_Subscribe(t *testing.T) {
	t.Run("should return duplicate subscription error on existing id", func(t *testing.T) {
		handler := func(_ string, _ ...any) error {
			return nil
		}

		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Subscribe("id", "channel", handler))
		assert.ErrorIs(t, ps.Subscribe("id", "channel", handler), flam.ErrDuplicateSubscription)
	})

	t.Run("should correctly subscribe the channel", func(t *testing.T) {
		var called bool
		handler := func(channel string, _ ...any) error {
			assert.Equal(t, "channel", channel)

			called = true
			return nil
		}

		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Subscribe("id", "channel", handler))
		assert.NoError(t, ps.Publish("channel"))

		assert.True(t, called)
	})
}

func Test_PubSub_Unsubscribe(t *testing.T) {
	t.Run("should unsubscribe channel", func(t *testing.T) {
		var called bool
		handler := func(channel string, _ ...any) error {
			assert.Equal(t, "channel", channel)

			called = true
			return nil
		}

		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Subscribe("id", "channel", handler))
		assert.NoError(t, ps.Unsubscribe("id", "channel"))
		assert.NoError(t, ps.Publish("channel"))

		assert.False(t, called)
	})

	t.Run("should return subscription not found when unsubscribe from non-existent channel", func(t *testing.T) {
		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.ErrorIs(t, ps.Unsubscribe("id", "channel"), flam.ErrSubscriptionNotFound)
	})

	t.Run("should return subscription not found when unsubscribe from non-existent id", func(t *testing.T) {
		handler := func(_ string, _ ...any) error {
			return nil
		}

		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Subscribe("id1", "channel", handler))
		assert.ErrorIs(t, ps.Unsubscribe("id2", "channel"), flam.ErrSubscriptionNotFound)
	})
}

func Test_PubSub_Publish(t *testing.T) {
	t.Run("should publish to channel with no subscribers without error", func(t *testing.T) {
		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Publish("channel", "data"))
	})

	t.Run("should publish with data without error", func(t *testing.T) {
		handler := func(channel string, data ...any) error {
			assert.Equal(t, "channel", channel)
			assert.Equal(t, []any{"hello", 123}, data)

			return nil
		}

		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Subscribe("id", "channel", handler))
		assert.NoError(t, ps.Publish("channel", "hello", 123))
	})

	t.Run("should return error if handler returns error", func(t *testing.T) {
		handler1 := func(channel string, data ...any) error {
			assert.Equal(t, "channel", channel)
			assert.Equal(t, "data", data[0])

			return nil
		}

		expectedErr := errors.New("handler error")
		handler2 := func(channel string, data ...any) error {
			assert.Equal(t, "channel", channel)
			assert.Equal(t, "data", data[0])

			return expectedErr
		}

		handler3 := func(channel string, data ...any) error {
			assert.Equal(t, "channel", channel)
			assert.Equal(t, "data", data[0])

			return nil
		}

		ps := flam.NewPubSub[string, string]()
		require.NotNil(t, ps)

		assert.NoError(t, ps.Subscribe("id1", "channel", handler1))
		assert.NoError(t, ps.Subscribe("id2", "channel", handler2))
		assert.NoError(t, ps.Subscribe("id3", "channel", handler3))

		assert.ErrorIs(t, ps.Publish("channel", "data"), expectedErr)
	})
}

func Test_PubSub_concurrency(t *testing.T) {
	var wg sync.WaitGroup
	subscriberCount := 100
	publishCount := 10

	var receivedCount int32
	handler := func(channel string, _ ...any) error {
		assert.Equal(t, "channel", channel)

		atomic.AddInt32(&receivedCount, 1)

		return nil
	}

	ps := flam.NewPubSub[string, string]()
	require.NotNil(t, ps)

	// Concurrent Subscribe
	for i := range subscriberCount {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			assert.NoError(t, ps.Subscribe(fmt.Sprintf("sub-%d", i), "channel", handler))
		}(i)
	}
	wg.Wait()

	// Concurrently event publish.
	for range publishCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, ps.Publish("channel"))
		}()
	}
	wg.Wait()

	// Concurrently event unsubscribe.
	for i := range subscriberCount {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			assert.NoError(t, ps.Unsubscribe(fmt.Sprintf("sub-%d", i), "channel"))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int32(subscriberCount*publishCount), atomic.LoadInt32(&receivedCount))
	atomic.StoreInt32(&receivedCount, 0)

	assert.NoError(t, ps.Publish("channel"))

	assert.Equal(t, int32(0), atomic.LoadInt32(&receivedCount))
}
