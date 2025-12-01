package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_LogSerializerFactory_Close(t *testing.T) {
	t.Run("should correctly close stored serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		logSerializerMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.NoError(t, factory.Store("my_serializer", logSerializerMock))
		}))

		assert.NoError(t, app.Close())
	})
}

func Test_LogSerializerFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerAlphaMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerAlphaMock.EXPECT().Close().Return(nil)

		logSerializerZuluMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerZuluMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			require.NoError(t, factory.Store("alpha", logSerializerAlphaMock))
			require.NoError(t, factory.Store("zulu", logSerializerZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added serializers and config defined serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		logSerializerCharlieMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerCharlieMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			require.NoError(t, factory.Store("charlie", logSerializerCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_LogSerializerFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated serializers", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.LogSerializerDriverJson},
			"alpha": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerMock1 := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock1.EXPECT().Close().Return(nil)

		logSerializerMock2 := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer_1", logSerializerMock1))
			require.NoError(t, factory.Store("my_serializer_2", logSerializerMock2))

			assert.Equal(t, []string{"my_serializer_1", "my_serializer_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.LogSerializerDriverJson},
			"alpha": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		logSerializerMock1 := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock1.EXPECT().Close().Return(nil)

		logSerializerMock2 := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_serializer_1", logSerializerMock1))
			require.NoError(t, factory.Store("my_serializer_2", logSerializerMock2))

			assert.Equal(t, []string{"alpha", "my_serializer_1", "my_serializer_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_LogSerializerFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathLogSerializers, flam.Bag{
		"ny_serializer_1": flam.Bag{
			"driver": flam.LogSerializerDriverJson}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	logSerializerMock := mocks.NewMockLogSerializer(ctrl)
	logSerializerMock.EXPECT().Close().Return(nil)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
		require.NoError(t, factory.Store("ny_serializer_2", logSerializerMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_serializer_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_serializer_2",
				expected: true},
			{
				name:     "non-existent entry",
				id:       "nonexistent",
				expected: false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, factory.Has(tc.id))
			})
		}
	}))
}

func Test_LogSerializerFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			got, e := factory.Get("my_serializer")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved serializer", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			got, e := factory.Get("my_serializer")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_serializer")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_LogSerializerFactory_Store(t *testing.T) {
	t.Run("should return nil reference if serializer is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.ErrorIs(t, factory.Store("my_serializer", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if serializer reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		logSerializerMock := mocks.NewMockLogSerializer(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.ErrorIs(t, factory.Store("my_serializer", logSerializerMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if serializer has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.NoError(t, factory.Store("my_serializer", logSerializerMock))
		}))
	})

	t.Run("should return duplicate resource if serializer has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.NoError(t, factory.Store("my_serializer", logSerializerMock))
			assert.ErrorIs(t, factory.Store("my_serializer", logSerializerMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_LogSerializerFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the serializer is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.ErrorIs(t, factory.Remove("my_serializer"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove serializer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", logSerializerMock))

			assert.NoError(t, factory.Remove("my_serializer"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_LogSerializerFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored serializers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerMock1 := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock1.EXPECT().Close().Return(nil)

		logSerializerMock2 := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock2.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer_1", logSerializerMock1))
			require.NoError(t, factory.Store("my_serializer_2", logSerializerMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
