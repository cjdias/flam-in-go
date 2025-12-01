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

func Test_ConfigObserver_Boot(t *testing.T) {
	t.Run("should not register a config check frequency config observer if not booted", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.False(t, config.HasObserver("flam.config", flam.PathConfigObserverFrequency))
		}))
	})

	t.Run("should return error when config observer has failed to create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigObserverFrequency, 10*time.Millisecond)

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

	t.Run("should return error when config observer has failed to be stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigObserverFrequency, 10*time.Millisecond)

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
			require.NoError(t, config.AddObserver("flam.config", flam.PathConfigObserverFrequency, func(old, new any) {}))
		}))

		require.ErrorIs(t, app.Boot(), flam.ErrDuplicateConfigObserver)
	})

	t.Run("should register a config check frequency config observer if booted", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.True(t, config.HasObserver("flam.config", flam.PathConfigObserverFrequency))
		}))
	})

	t.Run("should update the config check frequency observer trigger when the config check frequency changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigObserverFrequency, 10*time.Millisecond)

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
			assert.NoError(t, config.Set(flam.PathConfigObserverFrequency, 20*time.Millisecond))
		}))
	})

	t.Run("should not update the config check frequency observer trigger when the config check frequency is not a duration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigObserverFrequency, time.Millisecond)

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
			assert.NoError(t, config.Set(flam.PathConfigObserverFrequency, "string"))
		}))
	})

	t.Run("should reload sources if check frequency was booted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigObserverFrequency, time.Millisecond)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		wg := &sync.WaitGroup{}
		configSourceMock := mocks.NewMockObservableConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{}).Times(2)
		configSourceMock.EXPECT().GetPriority().Return(1)
		configSourceMock.EXPECT().Reload().DoAndReturn(func() (bool, error) {
			wg.Done()
			return false, nil
		}).Times(3)
		configSourceMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) error {
			return factory.Store("my_source", configSourceMock)
		}))

		require.NoError(t, app.Boot())

		wg.Add(1)
		wg.Wait() // load

		wg.Add(1)
		wg.Wait() // reload on trigger

		wg.Add(1)
		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.NoError(t, config.Set(flam.PathConfigObserverFrequency, 20*time.Millisecond))
		}))
		wg.Wait() // reload on frequency change
	})
}
