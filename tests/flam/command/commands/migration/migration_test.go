package migration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/command"
	"github.com/cjdias/flam-in-go/pkg/flam/command/commands"
	"github.com/cjdias/flam-in-go/pkg/flam/command/commands/migration"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flammigration "github.com/cjdias/flam-in-go/pkg/flam/migration"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var (
	errBoom = errors.New("boom")
	ts      = time.Date(2026, 11, 4, 10, 0, 0, 0, time.FixedZone("CET", 3600))
)

// migrator is a migrator returning the values set, and recording its
// calls.
type migrator struct {
	flam.IMigrator

	id      string
	applied []flam.MigrationInfo
	down    *flam.MigrationInfo
	list    []flam.MigrationInfo
	err     error
	calls   []string
}

func (m *migrator) Id() string { return m.id }

func (m *migrator) UpAll() ([]flam.MigrationInfo, error) {
	m.calls = append(m.calls, "up")
	return m.applied, m.err
}

func (m *migrator) Down() (*flam.MigrationInfo, error) {
	m.calls = append(m.calls, "down")
	return m.down, m.err
}

func (m *migrator) List() ([]flam.MigrationInfo, error) {
	m.calls = append(m.calls, "list")
	return m.list, m.err
}

// migrators is a migrator factory of its migrators, by id.
type migrators struct {
	flam.IMigratorFactory

	all map[string]*migrator
}

// newMigrators returns the factory of the migrators.
func newMigrators(all ...*migrator) *migrators {
	f := &migrators{all: map[string]*migrator{}}
	for _, m := range all {
		f.all[m.id] = m
	}
	return f
}

func (f *migrators) Available() []string {
	ids := []string{}
	for _, id := range []string{"a", "b", "c"} {
		if f.all[id] != nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (f *migrators) Get(id string) (flam.IMigrator, error) {
	if m, ok := f.all[id]; ok {
		return m, nil
	}
	return nil, errBoom
}

// failingWriter is a writer failing every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errBoom }

// info returns the information of the migration of the id.
func info(id string, installed bool) flam.MigrationInfo {
	i := flam.MigrationInfo{Id: id, Description: "migration " + id}
	if installed {
		at := ts
		i.InstalledAt = &at
	}
	return i
}

// newCommand returns the migrate command of the bundle over the factory.
func newCommand(t *testing.T, f flam.IMigratorFactory) flam.ICommand {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.Provide(func() flam.IMigratorFactory { return f }))
	testutil.NoError(t, a.RegisterBundle(migration.NewBundle()))
	var c flam.ICommand
	testutil.NoError(t, a.Invoke(func(p struct {
		dig.In

		Commands []flam.ICommand `group:"flam.command.commands"`
	}) {
		c = p.Commands[0]
	}))
	return c
}

// runCommand runs the command over the factory with the arguments,
// returning its output and error.
func runCommand(t *testing.T, f flam.IMigratorFactory, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	err := newCommand(t, f).Run(context.Background(), args, out)
	return out.String(), err
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "Name", migration.Name, "migrate")
	testutil.Equal(t, "ActionUp", migration.ActionUp, "up")
	testutil.Equal(t, "ActionDown", migration.ActionDown, "down")
	testutil.Equal(t, "ActionStatus", migration.ActionStatus, "status")
	testutil.Equal(t, "ErrorFieldAction", migration.ErrorFieldAction, "action")
	testutil.Equal(t, "ErrorFieldID", migration.ErrorFieldID, "id")
	if migration.ErrInvalidAction == nil {
		t.Error("ErrInvalidAction = nil")
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the command in the command group", func(t *testing.T) {
		b := migration.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.command.commands.migration")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.command.commands.migration", "provider/1"})

		c := newCommand(t, newMigrators())
		testutil.Equal(t, "Name()", c.Name(), "migrate")
		testutil.Equal(t, "Description()", c.Description(),
			"apply (up, the default), revert the last (down) or list (status) the migrations of the migrators, all or those named: migrate [up|down|status] [migrator...]")
	})

	t.Run("should register every command bundle", func(t *testing.T) {
		b := commands.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.command.commands")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.command.commands", "bundle:flam.command.commands.migration", "provider/1",
		})
	})
}

func Test_Up(t *testing.T) {
	t.Run("should apply the pending migrations of every migrator", func(t *testing.T) {
		a := &migrator{id: "a", applied: []flam.MigrationInfo{info("1", true), info("2", true)}}
		b := &migrator{id: "b"}
		for name, args := range map[string][]string{"default": nil, "up": {"up"}} {
			a.calls, b.calls = nil, nil
			out, err := runCommand(t, newMigrators(a, b), args...)
			testutil.NoError(t, err)
			testutil.Equal(t, name+" output", out, "a: applied 1 (migration 1)\na: applied 2 (migration 2)\nb: up to date\n")
			testutil.Equal(t, name+" a calls", a.calls, []string{"up"})
			testutil.Equal(t, name+" b calls", b.calls, []string{"up"})
		}
	})

	t.Run("should apply the pending migrations of the named migrators", func(t *testing.T) {
		a, b, c := &migrator{id: "a"}, &migrator{id: "b"}, &migrator{id: "c"}
		out, err := runCommand(t, newMigrators(a, b, c), "up", "c", "a")
		testutil.NoError(t, err)
		testutil.Equal(t, "output", out, "c: up to date\na: up to date\n")
		testutil.Equal(t, "b calls", b.calls, []string(nil))
	})

	t.Run("should write the applied migrations and stop at a failure", func(t *testing.T) {
		a := &migrator{id: "a", applied: []flam.MigrationInfo{info("1", true)}, err: errBoom}
		b := &migrator{id: "b"}
		out, err := runCommand(t, newMigrators(a, b))
		testutil.Error(t, err, errBoom, "unable to migrate: boom",
			map[string]any{migration.ErrorFieldAction: "up", migration.ErrorFieldID: "a"})
		testutil.Equal(t, "output", out, "a: applied 1 (migration 1)\n")
		testutil.Equal(t, "b calls", b.calls, []string(nil))
	})
}

func Test_Down(t *testing.T) {
	t.Run("should revert the last applied migration", func(t *testing.T) {
		i := info("2", false)
		a := &migrator{id: "a", down: &i}
		b := &migrator{id: "b", err: flammigration.ErrNoInstalled}
		out, err := runCommand(t, newMigrators(a, b), "down")
		testutil.NoError(t, err)
		testutil.Equal(t, "output", out, "a: reverted 2 (migration 2)\nb: nothing to revert\n")
		testutil.Equal(t, "a calls", a.calls, []string{"down"})
	})

	t.Run("should fail when a migrator fails", func(t *testing.T) {
		a := &migrator{id: "a", err: errBoom}
		out, err := runCommand(t, newMigrators(a), "down")
		testutil.Error(t, err, errBoom, "unable to migrate: boom",
			map[string]any{migration.ErrorFieldAction: "down", migration.ErrorFieldID: "a"})
		testutil.Equal(t, "output", out, "")
	})
}

func Test_Status(t *testing.T) {
	t.Run("should list the migrations", func(t *testing.T) {
		a := &migrator{id: "a", list: []flam.MigrationInfo{info("1", true), info("2", false)}}
		b := &migrator{id: "b"}
		out, err := runCommand(t, newMigrators(a, b), "status")
		testutil.NoError(t, err)
		testutil.Equal(t, "output", out, "a: 1 applied at 2026-11-04T09:00:00Z (migration 1)\n"+
			"a: 2 pending (migration 2)\n"+
			"b: no migration\n")
	})

	t.Run("should fail when a migrator fails", func(t *testing.T) {
		a := &migrator{id: "a", err: errBoom}
		_, err := runCommand(t, newMigrators(a), "status")
		testutil.Error(t, err, errBoom, "unable to migrate: boom",
			map[string]any{migration.ErrorFieldAction: "status", migration.ErrorFieldID: "a"})
	})
}

func Test_Run(t *testing.T) {
	t.Run("should fail for an invalid action", func(t *testing.T) {
		a := &migrator{id: "a"}
		_, err := runCommand(t, newMigrators(a), "sideways", "a")
		testutil.Error(t, err, migration.ErrInvalidAction, "unable to migrate: invalid migrate action",
			map[string]any{migration.ErrorFieldAction: "sideways"})
		testutil.Equal(t, "calls", a.calls, []string(nil))
	})

	t.Run("should fail for an unknown migrator", func(t *testing.T) {
		_, err := runCommand(t, newMigrators(), "status", "x")
		testutil.Error(t, err, errBoom, "unable to migrate: boom",
			map[string]any{migration.ErrorFieldAction: "status", migration.ErrorFieldID: "x"})
	})

	t.Run("should stop once the context is done", func(t *testing.T) {
		a := &migrator{id: "a"}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := newCommand(t, newMigrators(a)).Run(ctx, nil, io.Discard)
		testutil.Error(t, err, context.Canceled, "unable to migrate: context canceled",
			map[string]any{migration.ErrorFieldAction: "up", migration.ErrorFieldID: "a"})
		testutil.Equal(t, "calls", a.calls, []string(nil))
	})

	t.Run("should fail when the output fails", func(t *testing.T) {
		a := &migrator{id: "a"}
		err := newCommand(t, newMigrators(a)).Run(context.Background(), nil, failingWriter{})
		testutil.Error(t, err, errBoom, "unable to migrate: boom",
			map[string]any{migration.ErrorFieldAction: "up", migration.ErrorFieldID: "a"})
	})
}

func Test_Application(t *testing.T) {
	// table returns the migration of the id creating, and dropping, its
	// table.
	table := func(id string) flam.IMigration {
		return flammigration.NewMigration(id, "table "+id, "main", nil,
			func(c flam.IDatabaseConnection) error {
				return c.Exec(fmt.Sprintf("CREATE TABLE t%s (id INTEGER)", id)).Error
			},
			func(c flam.IDatabaseConnection) error {
				return c.Exec(fmt.Sprintf("DROP TABLE t%s", id)).Error
			})
	}
	dbPath := filepath.Join(t.TempDir(), "app.db")

	// migrate runs the application with the arguments, and returns its
	// output.
	migrate := func(t *testing.T, args ...string) string {
		t.Helper()
		vals := container.Bag{}
		for path, value := range map[string]any{
			command.ArgsConfigPath: append([]string{"migrate"}, args...),
			database.DialectConfigPath + ".main": container.Bag{
				factory.ConfigFieldDriver: sqlite.Driver,
				sqlite.ConfigFieldPath:    dbPath,
			},
			database.ConnectionConfigPath + ".main":    container.Bag{database.ConfigFieldDialect: "main"},
			flammigration.MigratorConfigPath + ".main": container.Bag{flammigration.ConfigFieldConnection: "main"},
		} {
			testutil.NoError(t, vals.Set(path, value))
		}
		a := app.New(vals)
		for _, b := range []flam.IBundle{
			database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(), flammigration.NewBundle(),
			command.NewBundle(), commands.NewBundle(),
		} {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		for _, id := range []string{"1", "2"} {
			testutil.NoError(t, a.Provide(func() flam.IMigration { return table(id) }, dig.Group(flammigration.MigrationGroup)))
		}
		out := &bytes.Buffer{}
		testutil.NoError(t, a.Provide(func() io.Writer { return out }, dig.Name(command.OutputName)))
		testutil.NoError(t, a.Run())
		testutil.NoError(t, a.Close())
		return out.String()
	}

	testutil.Equal(t, "up", migrate(t, "up"), "main: applied 1 (table 1)\nmain: applied 2 (table 2)\n")
	testutil.Equal(t, "up again", migrate(t), "main: up to date\n")
	testutil.Equal(t, "down", migrate(t, "down"), "main: reverted 2 (table 2)\n")
	testutil.Equal(t, "down", migrate(t, "down", "main"), "main: reverted 1 (table 1)\n")
	testutil.Equal(t, "nothing", migrate(t, "down"), "main: nothing to revert\n")
	testutil.Equal(t, "status", migrate(t, "status"), "main: 1 pending (table 1)\nmain: 2 pending (table 2)\n")
}
