package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_MigratorFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added migrators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorAlphaMock := mocks.NewMockMigrator(ctrl)
		migratorZuluMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("alpha", migratorAlphaMock))
			require.NoError(t, factory.Store("zulu", migratorZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added migrators and config defined migrators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		migratorCharlieMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("charlie", migratorCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_MigratorFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated migrators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"zulu": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"},
			"alpha": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil).AnyTimes()
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added migrators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock1 := mocks.NewMockMigrator(ctrl)
		migratorMock2 := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_migrator_1", migratorMock1))
			require.NoError(t, factory.Store("my_migrator_2", migratorMock2))

			assert.Equal(t, []string{"my_migrator_1", "my_migrator_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated migrators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"zulu": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"},
			"alpha": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil).AnyTimes()
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migratorMock1 := mocks.NewMockMigrator(ctrl)
		migratorMock2 := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_migrator_1", migratorMock1))
			require.NoError(t, factory.Store("my_migrator_2", migratorMock2))

			assert.Equal(t, []string{"alpha", "my_migrator_1", "my_migrator_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_MigratorFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathMigrators, flam.Bag{
		"ny_migrator_1": flam.Bag{}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	migratorMock := mocks.NewMockMigrator(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
		require.NoError(t, factory.Store("ny_migrator_2", migratorMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_migrator_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_migrator_2",
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

func Test_MigratorFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			got, e := factory.Get("my_migrator")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved migrator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "my_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil).AnyTimes()
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			got, e := factory.Get("my_migrator")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_migrator")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_MigratorFactory_Store(t *testing.T) {
	t.Run("should return nil reference if migrator is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.ErrorIs(t, factory.Store("my_migrator", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if migrator reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.ErrorIs(t, factory.Store("my_migrator", migratorMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if migrator has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.NoError(t, factory.Store("my_migrator", migratorMock))
		}))
	})

	t.Run("should return duplicate resource if migrator has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.NoError(t, factory.Store("my_migrator", migratorMock))
			assert.ErrorIs(t, factory.Store("my_migrator", migratorMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_MigratorFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the migrator is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.ErrorIs(t, factory.Remove("my_migrator"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove migrator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_migrator", migratorMock))

			assert.NoError(t, factory.Remove("my_migrator"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_MigratorFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored migrators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock1 := mocks.NewMockMigrator(ctrl)
		migratorMock2 := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_migrator_1", migratorMock1))
			require.NoError(t, factory.Store("my_migrator_2", migratorMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
