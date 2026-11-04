package migration

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "MigratorConfigPath", migration.MigratorConfigPath, "flam.migration.migrators")
	testutil.Equal(t, "MigratorCreatorGroup", migration.MigratorCreatorGroup, "flam.migration.migrators.creator")
	testutil.Equal(t, "LoggerConfigPath", migration.LoggerConfigPath, "flam.migration.loggers")
	testutil.Equal(t, "LoggerCreatorGroup", migration.LoggerCreatorGroup, "flam.migration.loggers.creator")
	testutil.Equal(t, "MigrationGroup", migration.MigrationGroup, "flam.migration.migrations")
	testutil.Equal(t, "BootConfigPath", migration.BootConfigPath, "flam.migration.boot")
	testutil.Equal(t, "Driver", migration.Driver, "flam.migration.migrators.driver.standard")
	testutil.Equal(t, "RecordTable", migration.RecordTable, "__migrations")
	testutil.Equal(t, "UpFileSuffix", migration.UpFileSuffix, ".up.sql")
	testutil.Equal(t, "DownFileSuffix", migration.DownFileSuffix, ".down.sql")
}

func Test_NewMigration(t *testing.T) {
	t.Run("should hold its description, without sharing its dependencies", func(t *testing.T) {
		deps := []string{"a", "b"}
		m := migration.NewMigration("1", "first", "main", deps, nil, nil)
		deps[0] = "changed"

		testutil.Equal(t, "Id()", m.Id(), "1")
		testutil.Equal(t, "Description()", m.Description(), "first")
		testutil.Equal(t, "Group()", m.Group(), "main")
		got := m.Dependencies()
		testutil.Equal(t, "Dependencies()", got, []string{"a", "b"})
		got[0] = "changed"
		testutil.Equal(t, "Dependencies() again", m.Dependencies(), []string{"a", "b"})
	})

	t.Run("should call its functions with the connection", func(t *testing.T) {
		conn := openConnection(t)
		var calls []string
		m := migration.NewMigration("1", "", "main", nil,
			func(c flam.IDatabaseConnection) error { calls = append(calls, "up "+c.Id()); return nil },
			func(c flam.IDatabaseConnection) error { calls = append(calls, "down "+c.Id()); return errBoom })

		testutil.NoError(t, m.Up(conn))
		testutil.Equal(t, "Down()", m.Down(conn), errBoom)
		testutil.Equal(t, "calls", calls, []string{"up main", "down main"})
	})

	t.Run("should apply nothing without up, and be irreversible without down", func(t *testing.T) {
		m := migration.NewMigration("1", "", "main", nil, nil, nil)

		testutil.NoError(t, m.Up(nil))
		testutil.Error(t, m.Down(nil), migration.ErrIrreversible, "unable to revert migration: irreversible migration",
			map[string]any{"migration": "1"})
	})
}

func Test_NewFileMigration(t *testing.T) {
	files := fstest.MapFS{
		"sql/001.up.sql":   {Data: []byte("CREATE TABLE a (id INTEGER);\nCREATE TABLE b (id INTEGER);")},
		"sql/001.down.sql": {Data: []byte("DROP TABLE a; DROP TABLE b;")},
		"sql/002.up.sql":   {Data: []byte("  \n")},
		"sql/003.up.sql":   {Data: []byte("NOT SQL")},
	}

	t.Run("should run the statements of its files", func(t *testing.T) {
		conn := openConnection(t)
		m := migration.NewFileMigration("001", "tables", "main", []string{"000"}, files, "sql/001")
		testutil.Equal(t, "Description()", m.Description(), "tables")
		testutil.Equal(t, "Dependencies()", m.Dependencies(), []string{"000"})

		testutil.NoError(t, m.Up(conn))
		testutil.Equal(t, "a", conn.Migrator().HasTable("a"), true)
		testutil.Equal(t, "b", conn.Migrator().HasTable("b"), true)
		testutil.NoError(t, m.Down(conn))
		testutil.Equal(t, "a dropped", conn.Migrator().HasTable("a"), false)
		testutil.Equal(t, "b dropped", conn.Migrator().HasTable("b"), false)
	})

	t.Run("should run nothing for a blank file", func(t *testing.T) {
		testutil.NoError(t, migration.NewFileMigration("002", "", "main", nil, files, "sql/002").Up(nil))
	})

	t.Run("should fail with a missing file", func(t *testing.T) {
		err := migration.NewFileMigration("002", "", "main", nil, files, "sql/002").Down(nil)
		testutil.Error(t, err, fs.ErrNotExist, "unable to run migration file: unable to run migration file: open sql/002.down.sql: file does not exist",
			map[string]any{"migration": "002", "path": "sql/002.down.sql"})
		if !errors.Is(err, migration.ErrFile) {
			t.Fatalf("Down() = %v, want ErrFile", err)
		}
	})

	t.Run("should fail with a failing statement", func(t *testing.T) {
		err := migration.NewFileMigration("003", "", "main", nil, files, "sql/003").Up(openConnection(t))
		var e *flam.Error
		if !errors.Is(err, migration.ErrFile) || !errors.As(err, &e) {
			t.Fatalf("Up() = %v, want ErrFile", err)
		}
		testutil.Context(t, e, map[string]any{"migration": "003", "path": "sql/003.up.sql"})
	})

	t.Run("should be applied by a migrator", func(t *testing.T) {
		conn := openConnection(t)
		m := newMigrator(t, conn, nil, migration.NewFileMigration("001", "", "main", nil, files, "sql/001"))
		_, err := m.UpAll()
		testutil.NoError(t, err)
		_, err = m.DownAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "a", conn.Migrator().HasTable("a"), false)
	})
}
