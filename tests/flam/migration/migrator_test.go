package migration

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ts is the time of the clock of the migrators.
var ts = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

var errBoom = errors.New("boom")

// openConnection returns the connection "main" over a new SQLite file,
// closed at the end of the test.
func openConnection(t *testing.T) flam.IDatabaseConnection {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Discard})
	testutil.NoError(t, err)
	conn, err := database.NewConnection("main", db)
	testutil.NoError(t, err)
	t.Cleanup(func() {
		if pool, err := db.DB(); err == nil {
			_ = pool.Close()
		}
	})
	return conn
}

// closePool closes the pool of the connection, so that it fails.
func closePool(t *testing.T, conn flam.IDatabaseConnection) {
	t.Helper()
	pool, err := conn.DB()
	testutil.NoError(t, err)
	testutil.NoError(t, pool.Close())
}

// table returns the migration of the id, in the group "main", creating the
// table of its id when applied and dropping it when reverted.
func table(id string, dependencies ...string) flam.IMigration {
	return migration.NewMigration(id, "table "+id, "main", dependencies,
		func(c flam.IDatabaseConnection) error {
			return c.Exec(fmt.Sprintf("CREATE TABLE t%s (id INTEGER)", id)).Error
		},
		func(c flam.IDatabaseConnection) error {
			return c.Exec(fmt.Sprintf("DROP TABLE t%s", id)).Error
		})
}

// hasTable reports whether the table of the migration id exists.
func hasTable(conn flam.IDatabaseConnection, id string) bool {
	return conn.Migrator().HasTable("t" + id)
}

// reports is a flam.IMigrationLoggerAdaptor recording its reports.
type reports struct {
	calls []string
}

func (r *reports) Id() string { return "reports" }

func (r *reports) UpStart(i flam.MigrationInfo) { r.calls = append(r.calls, "up start "+i.Id) }
func (r *reports) UpError(i flam.MigrationInfo, e error) {
	r.calls = append(r.calls, "up error "+i.Id+": "+e.Error())
}
func (r *reports) UpDone(i flam.MigrationInfo)    { r.calls = append(r.calls, "up done "+i.Id) }
func (r *reports) DownStart(i flam.MigrationInfo) { r.calls = append(r.calls, "down start "+i.Id) }
func (r *reports) DownError(i flam.MigrationInfo, e error) {
	r.calls = append(r.calls, "down error "+i.Id+": "+e.Error())
}
func (r *reports) DownDone(i flam.MigrationInfo) { r.calls = append(r.calls, "down done "+i.Id) }

// newMigrator returns the migrator "mig" of the group "main" and the
// migrations, over the connection, with the reports adaptor.
func newMigrator(t *testing.T, conn flam.IDatabaseConnection, r *reports, migrations ...flam.IMigration) flam.IMigrator {
	t.Helper()
	var adaptor flam.IMigrationLoggerAdaptor
	if r != nil {
		adaptor = r
	}
	m, err := migration.NewMigrator("mig", "main", migrations, conn, adaptor, testutil.NewClock(ts))
	testutil.NoError(t, err)
	return m
}

// ids returns the ids of the infos.
func ids(infos []flam.MigrationInfo) []string {
	out := []string{}
	for _, info := range infos {
		out = append(out, info.Id)
	}
	return out
}

func Test_NewMigrator(t *testing.T) {
	clock := testutil.NewClock(ts)
	var nilMigration *struct{ flam.IMigration }

	cases := []struct {
		name       string
		migrations []flam.IMigration
		conn       bool
		clock      flam.IClock
		sentinel   error
		text       string
		ctx        map[string]any
	}{
		{"nil connection", nil, false, clock, migration.ErrNilConnection, "nil migrator connection",
			map[string]any{"id": "mig", "group": "main"}},
		{"nil clock", nil, true, nil, migration.ErrNilClock, "nil migrator clock",
			map[string]any{"id": "mig", "group": "main"}},
		{"nil migration", []flam.IMigration{table("1"), nilMigration}, true, clock, migration.ErrNilMigration, "nil migration",
			map[string]any{"id": "mig", "group": "main"}},
		{"foreign group", []flam.IMigration{migration.NewMigration("1", "", "other", nil, nil, nil)}, true, clock,
			migration.ErrForeignGroup, "migration of another group",
			map[string]any{"id": "mig", "group": "main", "migration": "1"}},
		{"duplicate", []flam.IMigration{table("1"), table("1")}, true, clock,
			migration.ErrDuplicateMigration, "duplicate migration id",
			map[string]any{"id": "mig", "group": "main", "migration": "1"}},
		{"unknown dependency", []flam.IMigration{table("1", "0")}, true, clock,
			migration.ErrUnknownDependency, "unknown migration dependency",
			map[string]any{"id": "mig", "group": "main", "migration": "1", "dependency": "0"}},
		{"cycle", []flam.IMigration{table("1"), table("2", "3"), table("3", "2"), table("4", "3"), table("5", "5")}, true, clock,
			migration.ErrDependencyCycle, "migration dependency cycle",
			map[string]any{"id": "mig", "group": "main", "migrations": []string{"2", "3", "4", "5"}}},
	}
	for _, c := range cases {
		t.Run("should fail with a "+c.name, func(t *testing.T) {
			var conn flam.IDatabaseConnection
			if c.conn {
				conn = openConnection(t)
			}
			m, err := migration.NewMigrator("mig", "main", c.migrations, conn, nil, c.clock)
			testutil.Equal(t, "migrator", m, flam.IMigrator(nil))
			testutil.Error(t, err, c.sentinel, "unable to create migrator: "+c.text, c.ctx)
		})
	}

	t.Run("should fail when the records table cannot be created", func(t *testing.T) {
		conn := openConnection(t)
		closePool(t, conn)
		_, err := migration.NewMigrator("mig", "main", nil, conn, nil, clock)
		if !errors.Is(err, migration.ErrRecords) {
			t.Fatalf("NewMigrator() = %v, want ErrRecords", err)
		}
	})

	t.Run("should create the records table and treat a typed nil adaptor as none", func(t *testing.T) {
		conn := openConnection(t)
		var adaptor *reports
		m, err := migration.NewMigrator("mig", "main", []flam.IMigration{table("1")}, conn, adaptor, clock)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", m.Id(), "mig")
		testutil.Equal(t, "table", conn.Migrator().HasTable(migration.RecordTable), true)
		_, err = m.Up()
		testutil.NoError(t, err)
	})
}

func Test_Migrator(t *testing.T) {
	t.Run("should order by dependencies, then by id", func(t *testing.T) {
		conn := openConnection(t)
		m := newMigrator(t, conn, nil, table("5"), table("1", "4"), table("3"), table("4", "3"), table("2", "4", "4"))

		for range 5 {
			infos, err := m.List()
			testutil.NoError(t, err)
			testutil.Equal(t, "List()", ids(infos), []string{"3", "4", "1", "2", "5"})
			testutil.Equal(t, "Description", infos[0].Description, "table 3")
			testutil.Equal(t, "InstalledAt", infos[0].InstalledAt, (*time.Time)(nil))
		}
	})

	t.Run("should apply and revert one by one", func(t *testing.T) {
		conn := openConnection(t)
		r := &reports{}
		m := newMigrator(t, conn, r, table("2", "1"), table("1"))

		testutil.Equal(t, "CanUp()", m.CanUp(), true)
		testutil.Equal(t, "CanDown()", m.CanDown(), false)

		info, err := m.Up()
		testutil.NoError(t, err)
		testutil.Equal(t, "Up() Id", info.Id, "1")
		testutil.Equal(t, "Up() InstalledAt", *info.InstalledAt, ts)
		testutil.Equal(t, "t1", hasTable(conn, "1"), true)
		testutil.Equal(t, "CanDown()", m.CanDown(), true)

		infos, err := m.List()
		testutil.NoError(t, err)
		testutil.Equal(t, "installed", infos[0].InstalledAt.Equal(ts), true)
		testutil.Equal(t, "pending", infos[1].InstalledAt, (*time.Time)(nil))

		info, err = m.Up()
		testutil.NoError(t, err)
		testutil.Equal(t, "Up() Id", info.Id, "2")
		testutil.Equal(t, "CanUp()", m.CanUp(), false)
		_, err = m.Up()
		testutil.Error(t, err, migration.ErrNoPending, "unable to apply migrations: no pending migration",
			map[string]any{"id": "mig", "group": "main"})

		info, err = m.Down()
		testutil.NoError(t, err)
		testutil.Equal(t, "Down() Id", info.Id, "2")
		testutil.Equal(t, "Down() InstalledAt", info.InstalledAt, (*time.Time)(nil))
		testutil.Equal(t, "t2", hasTable(conn, "2"), false)
		_, err = m.Down()
		testutil.NoError(t, err)
		_, err = m.Down()
		testutil.Error(t, err, migration.ErrNoInstalled, "unable to revert migrations: no installed migration",
			map[string]any{"id": "mig", "group": "main"})

		testutil.Equal(t, "reports", r.calls, []string{
			"up start 1", "up done 1", "up start 2", "up done 2",
			"down start 2", "down done 2", "down start 1", "down done 1",
		})
	})

	t.Run("should apply all and revert all, the last applied first", func(t *testing.T) {
		conn := openConnection(t)
		m := newMigrator(t, conn, nil, table("c"), table("a", "c"), table("b"))

		infos, err := m.UpAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "UpAll()", ids(infos), []string{"b", "c", "a"})
		infos, err = m.UpAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "UpAll() again", ids(infos), []string{})

		// every migration has the same install time: the sequence orders them
		infos, err = m.DownAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "DownAll()", ids(infos), []string{"a", "c", "b"})
		infos, err = m.DownAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "DownAll() again", ids(infos), []string{})
	})

	t.Run("should keep the groups of a database apart", func(t *testing.T) {
		conn := openConnection(t)
		clock := testutil.NewClock(ts)
		first, err := migration.NewMigrator("first", "main", []flam.IMigration{table("1")}, conn, nil, clock)
		testutil.NoError(t, err)
		other := migration.NewMigration("1", "other 1", "other", nil, nil,
			func(flam.IDatabaseConnection) error { return nil })
		second, err := migration.NewMigrator("second", "other", []flam.IMigration{other}, conn, nil, clock)
		testutil.NoError(t, err)

		_, err = first.UpAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "second CanUp()", second.CanUp(), true)
		testutil.Equal(t, "second CanDown()", second.CanDown(), false)
		_, err = second.UpAll()
		testutil.NoError(t, err)

		info, err := second.Down()
		testutil.NoError(t, err)
		testutil.Equal(t, "Down() Id", info.Id, "1")
		testutil.Equal(t, "first CanDown()", first.CanDown(), true)
		testutil.Equal(t, "t1", hasTable(conn, "1"), true)
	})

	t.Run("should roll back a failing migration and stop", func(t *testing.T) {
		conn := openConnection(t)
		r := &reports{}
		failing := migration.NewMigration("2", "failing", "main", nil,
			func(c flam.IDatabaseConnection) error {
				if err := c.Exec("CREATE TABLE t2 (id INTEGER)").Error; err != nil {
					return err
				}
				return errBoom
			}, nil)
		m := newMigrator(t, conn, r, table("1"), failing, table("3"))

		infos, err := m.UpAll()
		testutil.Equal(t, "UpAll()", ids(infos), []string{"1"})
		testutil.Error(t, err, errBoom, "unable to apply migrations: unable to apply migration: boom",
			map[string]any{"id": "mig", "group": "main", "migration": "2"})
		if !errors.Is(err, migration.ErrUp) {
			t.Fatalf("UpAll() = %v, want ErrUp", err)
		}
		testutil.Equal(t, "t2 rolled back", hasTable(conn, "2"), false)
		testutil.Equal(t, "reports", r.calls[2:], []string{"up start 2", "up error 2: boom"})

		listed, err := m.List()
		testutil.NoError(t, err)
		testutil.Equal(t, "2 pending", listed[1].InstalledAt, (*time.Time)(nil))
	})

	t.Run("should fail to revert a failing or irreversible migration", func(t *testing.T) {
		conn := openConnection(t)
		r := &reports{}
		m := newMigrator(t, conn, r, table("1"), migration.NewMigration("2", "", "main", nil, nil, nil))
		_, err := m.UpAll()
		testutil.NoError(t, err)

		infos, err := m.DownAll()
		testutil.Equal(t, "DownAll()", ids(infos), []string{})
		if !errors.Is(err, migration.ErrDown) || !errors.Is(err, migration.ErrIrreversible) {
			t.Fatalf("DownAll() = %v, want ErrDown and ErrIrreversible", err)
		}
		testutil.Equal(t, "CanDown()", m.CanDown(), true)
		testutil.Equal(t, "reports", r.calls[len(r.calls)-1][:12], "down error 2")
	})

	t.Run("should fail to revert a migration it does not know", func(t *testing.T) {
		conn := openConnection(t)
		_, err := newMigrator(t, conn, nil, table("1"), table("2")).UpAll()
		testutil.NoError(t, err)

		_, err = newMigrator(t, conn, nil, table("1")).Down()
		testutil.Error(t, err, migration.ErrUnknownMigration, "unable to revert migrations: unknown installed migration",
			map[string]any{"id": "mig", "group": "main", "migration": "2"})
	})

	t.Run("should fail when the records cannot be written", func(t *testing.T) {
		conn := openConnection(t)
		dropUp := migration.NewMigration("1", "", "main", nil,
			func(c flam.IDatabaseConnection) error { return c.Migrator().DropTable(migration.RecordTable) }, nil)
		_, err := newMigrator(t, conn, nil, dropUp).Up()
		if !errors.Is(err, migration.ErrUp) {
			t.Fatalf("Up() = %v, want ErrUp", err)
		}

		conn = openConnection(t)
		dropDown := migration.NewMigration("1", "", "main", nil, nil,
			func(c flam.IDatabaseConnection) error { return c.Migrator().DropTable(migration.RecordTable) })
		m := newMigrator(t, conn, nil, dropDown)
		_, err = m.Up()
		testutil.NoError(t, err)
		_, err = m.Down()
		if !errors.Is(err, migration.ErrDown) {
			t.Fatalf("Down() = %v, want ErrDown", err)
		}

		conn = openConnection(t)
		dropAfter := migration.NewMigration("2", "", "main", nil, nil,
			func(c flam.IDatabaseConnection) error { return c.Migrator().DropTable(migration.RecordTable) })
		m = newMigrator(t, conn, nil, table("1"), dropAfter)
		_, err = m.UpAll()
		testutil.NoError(t, err)
		_, err = m.DownAll()
		if !errors.Is(err, migration.ErrDown) {
			t.Fatalf("DownAll() = %v, want ErrDown", err)
		}
	})

	t.Run("should fail when the records cannot be read", func(t *testing.T) {
		conn := openConnection(t)
		m := newMigrator(t, conn, nil, table("1"))
		closePool(t, conn)

		_, err := m.List()
		testutil.Equal(t, "List()", errors.Is(err, migration.ErrRecords), true)
		testutil.Equal(t, "CanUp()", m.CanUp(), false)
		testutil.Equal(t, "CanDown()", m.CanDown(), false)
		_, err = m.Up()
		testutil.Equal(t, "Up()", errors.Is(err, migration.ErrRecords), true)
		_, err = m.UpAll()
		testutil.Equal(t, "UpAll()", errors.Is(err, migration.ErrRecords), true)
		_, err = m.Down()
		testutil.Equal(t, "Down()", errors.Is(err, migration.ErrRecords), true)
		_, err = m.DownAll()
		testutil.Equal(t, "DownAll()", errors.Is(err, migration.ErrRecords), true)
	})
}
