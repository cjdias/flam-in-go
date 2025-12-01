package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DiskFactory_Close(t *testing.T) {
	t.Run("should correctly close stored disks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.NoError(t, factory.Store("my_resource", mocks.NewMockDisk(ctrl)))
		}))

		assert.NoError(t, app.Close())
	})
}

func Test_DiskFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added disks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("alpha", mocks.NewMockDisk(ctrl)))
			require.NoError(t, factory.Store("zulu", mocks.NewMockDisk(ctrl)))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added disks and config defined disks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("charlie", mocks.NewMockDisk(ctrl)))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_DiskFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated disks", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.DiskDriverMemory},
			"alpha": flam.Bag{
				"driver": flam.DiskDriverOS}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added disks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk_1", mocks.NewMockDisk(ctrl)))
			require.NoError(t, factory.Store("my_disk_2", mocks.NewMockDisk(ctrl)))

			assert.Equal(t, []string{"my_disk_1", "my_disk_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated disks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.DiskDriverMemory},
			"alpha": flam.Bag{
				"driver": flam.DiskDriverOS}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_disk_1", mocks.NewMockDisk(ctrl)))
			require.NoError(t, factory.Store("my_disk_2", mocks.NewMockDisk(ctrl)))

			assert.Equal(t, []string{"alpha", "my_disk_1", "my_disk_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_DiskFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathDisks, flam.Bag{
		"my_disk_1": flam.Bag{
			"driver": flam.DiskDriverMemory}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
		require.NoError(t, factory.Store("my_disk_2", mocks.NewMockDisk(ctrl)))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "my_disk_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "my_disk_2",
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

func Test_DiskFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			got, e := factory.Get("my_disk")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved disk", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			got, e := factory.Get("my_disk")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_disk")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_DiskFactory_Store(t *testing.T) {
	t.Run("should return nil reference if disk is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.ErrorIs(t, factory.Store("my_disk", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if disk reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.ErrorIs(t, factory.Store("my_disk", mocks.NewMockDisk(ctrl)), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if disk has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.NoError(t, factory.Store("my_disk", mocks.NewMockDisk(ctrl)))
		}))
	})

	t.Run("should return duplicate resource if disk has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.NoError(t, factory.Store("my_disk", mocks.NewMockDisk(ctrl)))
			assert.ErrorIs(t, factory.Store("my_disk", mocks.NewMockDisk(ctrl)), flam.ErrDuplicateResource)
		}))
	})
}

func Test_DiskFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the disk is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.ErrorIs(t, factory.Remove("my_disk"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove disk", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", mocks.NewMockDisk(ctrl)))

			assert.NoError(t, factory.Remove("my_disk"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_DiskFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored disks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk_1", mocks.NewMockDisk(ctrl)))
			require.NoError(t, factory.Store("my_disk_2", mocks.NewMockDisk(ctrl)))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
