package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_Application_NewApplication(t *testing.T) {
	assert.NotNil(t, flam.NewApplication())
}

func Test_Application_Container(t *testing.T) {
	assert.NotNil(t, flam.NewApplication().Container())
}

func Test_Application_HasProvider(t *testing.T) {
	t.Run("should return false in an unknown provider", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.False(t, app.HasProvider("unknown"))
	})

	t.Run("should return true on the default app provider", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.True(t, app.HasProvider("flam.provider"))
	})

	t.Run("should return true on a registered provider", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		providerMock := mocks.NewMockProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(providerMock))

		assert.True(t, app.HasProvider("provider"))
	})
}

func Test_Application_Register(t *testing.T) {
	t.Run("should return nil reference error when provider is nil", func(t *testing.T) {
		assert.ErrorIs(t, flam.NewApplication().Register(nil), flam.ErrNilReference)
	})

	t.Run("should return duplicate provider error when provider is already registered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		providerMock := mocks.NewMockProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)

		assert.NoError(t, app.Register(providerMock))
		assert.ErrorIs(t, app.Register(providerMock), flam.ErrDuplicateProvider)
	})

	t.Run("should return an error if provider registration fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		expectedErr := errors.New("registration error")
		providerMock := mocks.NewMockProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider")
		providerMock.EXPECT().Register(gomock.Any()).Return(expectedErr)

		assert.ErrorIs(t, flam.NewApplication().Register(providerMock), expectedErr)
	})

	t.Run("should register a provider successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		providerMock := mocks.NewMockProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider")
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)

		assert.NoError(t, flam.NewApplication().Register(providerMock))
	})
}

func Test_Application_Boot(t *testing.T) {
	t.Run("should no-op if already booted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		providerMock := mocks.NewMockBootableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Boot(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(providerMock))

		assert.NoError(t, app.Boot())
		assert.NoError(t, app.Boot())
	})

	t.Run("should return an error if a configurable provider fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("config error")
		providerMock := mocks.NewMockConfigurableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Config(gomock.Any()).Return(expectedErr)

		require.NoError(t, app.Register(providerMock))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should configure all providers successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		provider1Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Config(gomock.Any()).Return(nil)

		provider2Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Config(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(provider1Mock))
		require.NoError(t, app.Register(provider2Mock))

		assert.NoError(t, app.Boot())
	})

	t.Run("should return an error if a source has already been registered with the same app source id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) error {
			return factory.Store(flam.DefaultConfigSourceId, configSourceMock)
		}))

		assert.ErrorIs(t, app.Boot(), flam.ErrDuplicateResource)
	})

	t.Run("should return an error if a bootable provider fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("boot error")
		providerMock := mocks.NewMockBootableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Boot(gomock.Any()).Return(expectedErr)

		require.NoError(t, app.Register(providerMock))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should boot all providers successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		provider1Mock := mocks.NewMockBootableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Boot(gomock.Any()).Return(nil)

		provider2Mock := mocks.NewMockBootableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Boot(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(provider1Mock))
		require.NoError(t, app.Register(provider2Mock))

		assert.NoError(t, app.Boot())
	})

	t.Run("should add app config to config facade on boot", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(flam.Bag{"key": "value"})
		defer func() { _ = app.Close() }()

		provider1Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Config(gomock.Any()).Return(nil)

		provider2Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Config(gomock.Any()).Return(nil)

		assert.NoError(t, app.Register(provider1Mock))
		assert.NoError(t, app.Register(provider2Mock))
		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.Equal(t, "value", config.Get("key"))
		}))
	})

	t.Run("should add provider config to config facade on boot", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(flam.Bag{"key1": "value1"})
		defer func() { _ = app.Close() }()

		provider1Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Config(gomock.Any()).DoAndReturn(func(bag *flam.Bag) {
			_ = bag.Set("key2", "value2")
		}).Return(nil)

		provider2Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Config(gomock.Any()).Return(nil)

		assert.NoError(t, app.Register(provider1Mock))
		assert.NoError(t, app.Register(provider2Mock))
		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.Equal(t, "value1", config.Get("key1"))
			assert.Equal(t, "value2", config.Get("key2"))
		}))
	})

	t.Run("should override provider config with app config if collides", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(flam.Bag{"key": "value1"})
		defer func() { _ = app.Close() }()

		provider1Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Config(gomock.Any()).DoAndReturn(func(bag *flam.Bag) {
			_ = bag.Set("key", "value2")
		}).Return(nil)

		provider2Mock := mocks.NewMockConfigurableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Config(gomock.Any()).Return(nil)

		assert.NoError(t, app.Register(provider1Mock))
		assert.NoError(t, app.Register(provider2Mock))
		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.Equal(t, "value1", config.Get("key"))
		}))
	})
}

func Test_Application_Run(t *testing.T) {
	t.Run("should boot if not already booted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		providerMock := mocks.NewMockBootableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Boot(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(providerMock))

		assert.NoError(t, app.Run())
	})

	t.Run("should return boot error if any", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("boot error")
		providerMock := mocks.NewMockBootableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Boot(gomock.Any()).Return(expectedErr)

		require.NoError(t, app.Register(providerMock))

		assert.ErrorIs(t, app.Run(), expectedErr)
	})

	t.Run("should return an error if a runnable provider fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("run error")
		providerMock := mocks.NewMockRunnableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Run(gomock.Any()).Return(expectedErr)

		require.NoError(t, app.Register(providerMock))

		assert.ErrorIs(t, app.Run(), expectedErr)
	})

	t.Run("should run all providers successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		provider1Mock := mocks.NewMockRunnableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Run(gomock.Any()).Return(nil)

		provider2Mock := mocks.NewMockRunnableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Run(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(provider1Mock))
		require.NoError(t, app.Register(provider2Mock))

		assert.NoError(t, app.Run())
	})
}

func Test_Application_Close(t *testing.T) {
	t.Run("should return an error if a runnable provider fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		expectedErr := errors.New("close error")
		providerMock := mocks.NewMockClosableProvider(ctrl)
		providerMock.EXPECT().Id().Return("provider").AnyTimes()
		providerMock.EXPECT().Register(gomock.Any()).Return(nil)
		providerMock.EXPECT().Close(gomock.Any()).Return(expectedErr)

		require.NoError(t, app.Register(providerMock))

		assert.ErrorIs(t, app.Close(), expectedErr)
	})

	t.Run("should run all providers successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		provider1Mock := mocks.NewMockClosableProvider(ctrl)
		provider1Mock.EXPECT().Id().Return("provider1").AnyTimes()
		provider1Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider1Mock.EXPECT().Close(gomock.Any()).Return(nil)

		provider2Mock := mocks.NewMockClosableProvider(ctrl)
		provider2Mock.EXPECT().Id().Return("provider2").AnyTimes()
		provider2Mock.EXPECT().Register(gomock.Any()).Return(nil)
		provider2Mock.EXPECT().Close(gomock.Any()).Return(nil)

		require.NoError(t, app.Register(provider1Mock))
		require.NoError(t, app.Register(provider2Mock))

		assert.NoError(t, app.Close())
	})
}
