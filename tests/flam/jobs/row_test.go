package jobs

import (
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/jobs"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Migrate(t *testing.T) {
	t.Run("should create the table", func(t *testing.T) {
		e := newEnv(t, options{})
		testutil.NoError(t, jobs.Migrate(e.conn.Gorm()))
		testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(jobs.Table), true)
		testutil.NoError(t, jobs.Migrate(e.conn.Gorm()))
	})

	t.Run("should fail when the table cannot be created", func(t *testing.T) {
		e := newEnv(t, options{})
		testutil.NoError(t, e.conn.Exec("CREATE VIEW flam_jobs AS SELECT 1 AS id").Error)
		err := jobs.Migrate(e.conn.Gorm())
		var fe *flam.Error
		if !errors.Is(err, jobs.ErrMigrate) || !errors.As(err, &fe) {
			t.Fatalf("Migrate() = %v, want ErrMigrate", err)
		}
		testutil.Context(t, fe, map[string]any{})
	})
}

func Test_NewMigration(t *testing.T) {
	t.Run("should create and drop the table", func(t *testing.T) {
		e := newEnv(t, options{})
		m := jobs.NewMigration("001", "main", "000")
		testutil.Equal(t, "Id()", m.Id(), "001")
		testutil.Equal(t, "Group()", m.Group(), "main")
		testutil.Equal(t, "Dependencies()", m.Dependencies(), []string{"000"})
		testutil.Equal(t, "Description()", m.Description(), "create the flam_jobs table")

		testutil.NoError(t, m.Up(e.conn))
		testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(jobs.Table), true)
		testutil.NoError(t, m.Down(e.conn))
		testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(jobs.Table), false)
	})
}
