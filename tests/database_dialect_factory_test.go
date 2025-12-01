package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DatabaseDialectFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added dialects", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectAlphaMock := mocks.NewMockDatabaseDialect(ctrl)
		databaseDialectZuluMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			require.NoError(t, factory.Store("alpha", databaseDialectAlphaMock))
			require.NoError(t, factory.Store("zulu", databaseDialectZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added dialects and config defined dialects", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseDialectCharlieMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			require.NoError(t, factory.Store("charlie", databaseDialectCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_DatabaseDialectFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated dialects", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite},
			"alpha": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added dialects", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectMock1 := mocks.NewMockDatabaseDialect(ctrl)
		databaseDialectMock2 := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			require.NoError(t, factory.Store("my_dialect_1", databaseDialectMock1))
			require.NoError(t, factory.Store("my_dialect_2", databaseDialectMock2))

			assert.Equal(t, []string{"my_dialect_1", "my_dialect_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated dialects", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite},
			"alpha": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseDialectMock1 := mocks.NewMockDatabaseDialect(ctrl)
		databaseDialectMock2 := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_dialect_1", databaseDialectMock1))
			require.NoError(t, factory.Store("my_dialect_2", databaseDialectMock2))

			assert.Equal(t, []string{"alpha", "my_dialect_1", "my_dialect_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_DatabaseDialectFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
		"ny_dialect_1": flam.Bag{
			"driver": flam.DatabaseDialectDriverSqlite}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	databaseDialectMock := mocks.NewMockDatabaseDialect(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
		require.NoError(t, factory.Store("ny_dialect_2", databaseDialectMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_dialect_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_dialect_2",
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

func Test_DatabaseDialectFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("my_dialect")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved dialect", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			got, e := factory.Get("my_dialect")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_dialect")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_DatabaseDialectFactory_Store(t *testing.T) {
	t.Run("should return nil reference if dialect is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.ErrorIs(t, factory.Store("my_dialect", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if dialect reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseDialectMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.ErrorIs(t, factory.Store("my_dialect", databaseDialectMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if dialect has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.NoError(t, factory.Store("my_dialect", databaseDialectMock))
		}))
	})

	t.Run("should return duplicate resource if dialect has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.NoError(t, factory.Store("my_dialect", databaseDialectMock))
			assert.ErrorIs(t, factory.Store("my_dialect", databaseDialectMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_DatabaseDialectFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the dialect is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.ErrorIs(t, factory.Remove("my_dialect"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove dialect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			require.NoError(t, factory.Store("my_dialect", databaseDialectMock))

			assert.NoError(t, factory.Remove("my_dialect"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_DatabaseDialectFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored dialects", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectMock1 := mocks.NewMockDatabaseDialect(ctrl)
		databaseDialectMock2 := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			require.NoError(t, factory.Store("my_dialect_1", databaseDialectMock1))
			require.NoError(t, factory.Store("my_dialect_2", databaseDialectMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
