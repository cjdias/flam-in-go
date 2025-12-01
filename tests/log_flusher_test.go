package tests

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_LogFlusher_Boot(t *testing.T) {
	t.Run("should not register a log flusher config observer if not booted", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.False(t, config.HasObserver("flam.log", flam.PathLogFlusherFrequency))
		}))
	})

	t.Run("should return error when log flusher has failed to create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogFlusherFrequency, 10*time.Millisecond)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("error")
		triggerFactoryMock := mocks.NewMockTriggerFactory(ctrl)
		triggerFactoryMock.EXPECT().NewRecurring(10*time.Millisecond, gomock.Any()).Return(nil, expectedErr)
		require.NoError(t, app.Container().Decorate(func(flam.TriggerFactory) flam.TriggerFactory {
			return triggerFactoryMock
		}))

		require.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return error when log flusher has failed to be stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogFlusherFrequency, 10*time.Millisecond)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		triggerMock := mocks.NewMockTrigger(ctrl)
		triggerMock.EXPECT().Close().Return(nil)

		triggerFactoryMock := mocks.NewMockTriggerFactory(ctrl)
		triggerFactoryMock.EXPECT().NewRecurring(10*time.Millisecond, gomock.Any()).Return(triggerMock, nil)
		require.NoError(t, app.Container().Decorate(func(flam.TriggerFactory) flam.TriggerFactory {
			return triggerFactoryMock
		}))

		require.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.AddObserver("flam.log", flam.PathLogFlusherFrequency, func(old, new any) {}))
		}))

		require.ErrorIs(t, app.Boot(), flam.ErrDuplicateConfigObserver)
	})

	t.Run("should register a log flusher config observer if booted", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.True(t, config.HasObserver("flam.log", flam.PathLogFlusherFrequency))
		}))
	})

	t.Run("should update the log flusher observer trigger when the config flush changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogFlusherFrequency, 10*time.Millisecond)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		triggerMock := mocks.NewMockTrigger(ctrl)
		triggerFactoryMock := mocks.NewMockTriggerFactory(ctrl)
		require.NoError(t, app.Container().Decorate(func(flam.TriggerFactory) flam.TriggerFactory {
			return triggerFactoryMock
		}))

		gomock.InOrder(
			triggerFactoryMock.EXPECT().NewRecurring(10*time.Millisecond, gomock.Any()).Return(triggerMock, nil),
			triggerMock.EXPECT().Close().Return(nil),
			triggerFactoryMock.EXPECT().NewRecurring(20*time.Millisecond, gomock.Any()).Return(triggerMock, nil),
			triggerMock.EXPECT().Close().Return(nil),
		)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.NoError(t, config.Set(flam.PathLogFlusherFrequency, 20*time.Millisecond))
		}))
	})

	t.Run("should not update the log flusher trigger when the config flush is not a duration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogFlusherFrequency, time.Millisecond)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		triggerMock := mocks.NewMockTrigger(ctrl)
		triggerMock.EXPECT().Close().Return(nil)

		triggerFactoryMock := mocks.NewMockTriggerFactory(ctrl)
		triggerFactoryMock.EXPECT().NewRecurring(time.Millisecond, gomock.Any()).Return(triggerMock, nil)
		require.NoError(t, app.Container().Decorate(func(flam.TriggerFactory) flam.TriggerFactory {
			return triggerFactoryMock
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.NoError(t, config.Set(flam.PathLogFlusherFrequency, "string"))
		}))
	})

	t.Run("should reload sources if check frequency was booted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogFlusherFrequency, time.Millisecond)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		wg := &sync.WaitGroup{}
		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.
			EXPECT().
			Broadcast(gomock.Any(), flam.LogFatal, "message", gomock.Any()).
			DoAndReturn(func(_ time.Time, _ flam.LogLevel, _ string, _ flam.Bag) error {
				wg.Done()
				return nil
			}).
			Times(2)
		logStreamMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) error {
			return factory.Store("my_stream", logStreamMock)
		}))

		require.NoError(t, app.Boot())

		wg.Add(1)
		assert.NoError(t, app.Container().Invoke(func(logger flam.Logger) {
			logger.Broadcast(flam.LogFatal, "message")
		}))
		wg.Wait() // broadcast on trigger

		wg.Add(1)
		assert.NoError(t, app.Container().Invoke(func(logger flam.Logger, config flam.Config) {
			logger.Broadcast(flam.LogFatal, "message")

			assert.NoError(t, config.Set(flam.PathLogFlusherFrequency, 20*time.Millisecond))
		}))
		wg.Wait() // broadcast on frequency change
	})
}
