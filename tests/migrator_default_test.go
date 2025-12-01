package tests

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DefaultMigrator_List(t *testing.T) {
	t.Run("should return migration listing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.ExpectQuery("SELECT \\* FROM `__migrations`").WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			list, e := migrator.List()
			require.Nil(t, list)
			require.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return correctly populated migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		installedAt := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", installedAt, installedAt))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		expected := []flam.MigrationInfo{{
			Version:     "1.0.0",
			Description: "1.0.0-description",
			InstalledAt: &installedAt,
		}, {
			Version:     "2.0.0",
			Description: "2.0.0-description",
		}}

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			list, e := migrator.List()
			require.Equal(t, expected, list)
			require.NoError(t, e)
		}))
	})
}

func Test_DefaultMigrator_Current(t *testing.T) {
	t.Run("should return migration listing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.ExpectQuery("SELECT \\* FROM `__migrations`").WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			info, e := migrator.Current()
			require.Nil(t, info)
			require.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return nil of no migration was executed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			list, e := migrator.Current()
			require.Nil(t, list)
			require.NoError(t, e)
		}))
	})

	t.Run("should return current/last executed migration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		installedAt := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", installedAt, installedAt))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		expected := flam.MigrationInfo{
			Version:     "1.0.0",
			Description: "1.0.0-description",
			InstalledAt: &installedAt}

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			list, e := migrator.Current()
			require.Equal(t, &expected, list)
			require.NoError(t, e)
		}))
	})
}

func Test_DefaultMigrator_CanUp(t *testing.T) {
	t.Run("should return false on migration listing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.ExpectQuery("SELECT \\* FROM `__migrations`").WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.False(t, migrator.CanUp())
		}))
	})

	t.Run("should return false on empty list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.False(t, migrator.CanUp())
		}))
	})

	t.Run("should return false on all executed migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()).
				AddRow(2, "2.0.0", "2.0.0-description", time.Now(), time.Now()))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.False(t, migrator.CanUp())
		}))
	})

	t.Run("should return true if missing any migration to be executed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.True(t, migrator.CanUp())
		}))
	})
}

func Test_DefaultMigrator_CanDown(t *testing.T) {
	t.Run("should return false on migration listing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.ExpectQuery("SELECT \\* FROM `__migrations`").WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.False(t, migrator.CanDown())
		}))
	})

	t.Run("should return false on empty list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.False(t, migrator.CanDown())
		}))
	})

	t.Run("should return false on non-executed migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.False(t, migrator.CanDown())
		}))
	})

	t.Run("should return true if any migration has been executed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group2").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.True(t, migrator.CanDown())
		}))
	})
}

func Test_DefaultMigrator_Up(t *testing.T) {
	t.Run("should return last migration retrieving error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), expectedErr)
		}))
	})

	t.Run("should return migration not found error on empty migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), flam.ErrUnknownResource)
		}))
	})

	t.Run("should return migration not found error on fully executed migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(2, "2.0.0", "2.0.0-description", time.Now(), time.Now()))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), flam.ErrUnknownResource)
		}))
	})

	t.Run("should return transaction execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))

		expectedErr := errors.New("transaction error")
		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.EXPECT().Transaction(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), expectedErr)
		}))
	})

	t.Run("should return migration execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		expectedErr := errors.New("migration error")
		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), expectedErr)
		}))
	})

	t.Run("should return migration execution error with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		expectedErr := errors.New("migration error")
		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' up action started")
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "migration '2.0.0' up action error: migration error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), expectedErr)
		}))
	})

	t.Run("should return migration recording action error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnError(expectedErr)
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), expectedErr)
		}))
	})

	t.Run("should return migration recording action error with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnError(expectedErr)
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' up action started")
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "migration '2.0.0' up action error: migration error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Up(), expectedErr)
		}))
	})

	t.Run("should return no error on success migration (first migration)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(1, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.Up())
		}))
	})

	t.Run("should return no error on success migration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.Up())
		}))
	})

	t.Run("should return no error on success migration with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' up action started")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' up action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.Up())
		}))
	})
}

func Test_DefaultMigrator_UpAll(t *testing.T) {
	t.Run("should return last migration retrieving error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.UpAll(), expectedErr)
		}))
	})

	t.Run("should no-op in a empty migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.UpAll())
		}))
	})

	t.Run("should no-op in a fully executed migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(2, "2.0.0", "2.0.0-description", time.Now(), time.Now()))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.UpAll())
		}))
	})

	t.Run("should return migration execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		expectedErr := errors.New("migration error")
		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.UpAll(), expectedErr)
		}))
	})

	t.Run("should execute all pending migration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(3, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			}).Times(2)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration3Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.UpAll())
		}))
	})

	t.Run("should execute all pending migration with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("INSERT INTO `__migrations`").
			WillReturnResult(sqlmock.NewResult(3, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			}).Times(2)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration3Mock.EXPECT().Up(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' up action started")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' up action terminated")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '3.0.0' up action started")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '3.0.0' up action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.UpAll())
		}))
	})
}

func Test_DefaultMigrator_Down(t *testing.T) {
	t.Run("should return last migration retrieving error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), expectedErr)
		}))
	})

	t.Run("should return migration not found error on empty migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), flam.ErrUnknownResource)
		}))
	})

	t.Run("should return migration not found error on non-executed migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), flam.ErrUnknownResource)
		}))
	})

	t.Run("should return transaction execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))

		expectedErr := errors.New("transaction error")
		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.EXPECT().Transaction(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), expectedErr)
		}))
	})

	t.Run("should return migration execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		expectedErr := errors.New("migration error")
		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), expectedErr)
		}))
	})

	t.Run("should return migration execution error with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		expectedErr := errors.New("migration error")
		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action started")
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "migration '1.0.0' down action error: migration error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), expectedErr)
		}))
	})

	t.Run("should return migration recording action error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnError(expectedErr)
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), expectedErr)
		}))
	})

	t.Run("should return migration recording action error with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnError(expectedErr)
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action started")
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "migration '1.0.0' down action error: migration error")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.Down(), expectedErr)
		}))
	})

	t.Run("should return no error on success migration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.Down())
		}))
	})

	t.Run("should return no error on success migration with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action started")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.Down())
		}))
	})
}

func Test_DefaultMigrator_DownAll(t *testing.T) {
	t.Run("should no-op in a empty migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.DownAll())
		}))
	})

	t.Run("should return last migration retrieving error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("migration listing error")
		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnError(expectedErr)

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.DownAll(), expectedErr)
		}))
	})

	t.Run("should no-op in a non-executed migration list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.DownAll())
		}))
	})

	t.Run("should return migration rollback error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.ExpectRollback()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			})
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		expectedErr := errors.New("migration error")
		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.ErrorIs(t, migrator.DownAll(), expectedErr)
		}))
	})

	t.Run("should rollback all executed migrations", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(2, "2.0.0", "2.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnResult(sqlmock.NewResult(3, 1))
		dbMock.ExpectCommit()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db).Times(3)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			}).Times(2)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(nil)
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.DownAll())
		}))
	})

	t.Run("should rollback all executed migrations with logging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver":  flam.MigratorLoggerDriverDefault,
				"channel": "flam",
				"levels": flam.Bag{
					"start": flam.LogInfo,
					"error": flam.LogError,
					"done":  flam.LogInfo}}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"logger_id":     "my_logger",
				"group":         "group1"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		db, dbMock := SetupDatabase()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(2, "2.0.0", "2.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnResult(sqlmock.NewResult(2, 1))
		dbMock.ExpectCommit()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}).
				AddRow(1, "1.0.0", "1.0.0-description", time.Now(), time.Now()))
		dbMock.ExpectBegin()
		dbMock.
			ExpectExec("DELETE FROM `__migrations`").
			WillReturnResult(sqlmock.NewResult(3, 1))
		dbMock.ExpectCommit()
		dbMock.
			ExpectQuery("SELECT \\* FROM `__migrations`").
			WillReturnRows(sqlmock.
				NewRows([]string{"id", "version", "description", "created_at", "updated_at"}))

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)
		databaseConnectionMock.EXPECT().AutoMigrate(gomock.Any()).Return(nil)
		databaseConnectionMock.EXPECT().Order("created_at desc").Return(db).Times(3)
		databaseConnectionMock.
			EXPECT().
			Transaction(gomock.Any()).
			DoAndReturn(func(
				callback func(tx *gorm.DB) error,
				opts ...*sql.TxOptions) error {
				return callback(db)
			}).Times(2)
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", databaseConnectionMock))
		}))

		migration1Mock := mocks.NewMockMigration(ctrl)
		migration1Mock.EXPECT().Version().Return("2.0.0").AnyTimes()
		migration1Mock.EXPECT().Description().Return("2.0.0-description").AnyTimes()
		migration1Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration1Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration1Mock
		}, dig.Group(flam.MigrationGroup)))

		migration2Mock := mocks.NewMockMigration(ctrl)
		migration2Mock.EXPECT().Version().Return("1.0.0").AnyTimes()
		migration2Mock.EXPECT().Description().Return("1.0.0-description").AnyTimes()
		migration2Mock.EXPECT().Group().Return("group1").AnyTimes()
		migration2Mock.EXPECT().Down(gomock.Any()).Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration2Mock
		}, dig.Group(flam.MigrationGroup)))

		migration3Mock := mocks.NewMockMigration(ctrl)
		migration3Mock.EXPECT().Version().Return("3.0.0").AnyTimes()
		migration3Mock.EXPECT().Description().Return("3.0.0-description").AnyTimes()
		migration3Mock.EXPECT().Group().Return("group1").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Migration {
			return migration3Mock
		}, dig.Group(flam.MigrationGroup)))

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' down action started")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '2.0.0' down action terminated")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action started")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "migration '1.0.0' down action terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			migrator, e := factory.Get("my_migrator")
			require.NotNil(t, migrator)
			require.NoError(t, e)

			require.NoError(t, migrator.DownAll())
		}))
	})
}
