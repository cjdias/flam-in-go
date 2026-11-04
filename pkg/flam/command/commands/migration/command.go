package migration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	flammigration "github.com/cjdias/flam-in-go/pkg/flam/migration"
)

// newCommand returns the migrate command, running the migrators of the
// factory. It is a provider registered by the bundle returned from
// [NewBundle].
func newCommand(
	migrators flam.IMigratorFactory,
) flam.ICommand {
	return &migrateCommand{
		migrators: migrators,
	}
}

// migrateCommand is the migrate command, applying, reverting or listing the
// migrations of the migrators of its factory.
type migrateCommand struct {
	migrators flam.IMigratorFactory
}

// compile time check that *migrateCommand implements ICommand
var _ flam.ICommand = (*migrateCommand)(nil)

func (c *migrateCommand) Name() string {
	return Name
}

func (c *migrateCommand) Description() string {
	return description
}

// Run runs the action of the first argument, [ActionUp] by default, on
// the migrators named by the other arguments, or else on every migrator
// configured, in id order. It writes what each migrator did, and stops at
// the first migrator failing, or when the context is done.
func (c *migrateCommand) Run(
	ctx context.Context,
	args []string,
	out io.Writer,
) error {
	action := ActionUp
	if len(args) > 0 {
		action, args = args[0], args[1:]
	}
	run, ok := map[string]func(*strings.Builder, string, flam.IMigrator) error{
		ActionUp:     c.up,
		ActionDown:   c.down,
		ActionStatus: c.status,
	}[action]
	if !ok {
		return newErrAction(commandRunMessage, action)
	}

	ids := args
	if len(ids) == 0 {
		ids = c.migrators.Available()
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return newErrMigrator(err, commandRunMessage, action, id)
		}
		migrator, err := c.migrators.Get(id)
		if err == nil {
			var b strings.Builder
			err = run(&b, id, migrator)
			_, werr := io.WriteString(out, b.String())
			err = errors.Join(err, werr)
		}
		if err != nil {
			return newErrMigrator(err, commandRunMessage, action, id)
		}
	}
	return nil
}

// up applies the pending migrations of the migrator, writing the applied
// ones.
func (c *migrateCommand) up(
	b *strings.Builder,
	id string,
	migrator flam.IMigrator,
) error {
	applied, err := migrator.UpAll()
	for _, info := range applied {
		fmt.Fprintf(b, "%s: applied %s (%s)\n", id, info.Id, info.Description)
	}
	if err == nil && len(applied) == 0 {
		fmt.Fprintf(b, "%s: up to date\n", id)
	}
	return err
}

// down reverts the last applied migration of the migrator, writing it,
// or that there is none.
func (c *migrateCommand) down(
	b *strings.Builder,
	id string,
	migrator flam.IMigrator,
) error {
	info, err := migrator.Down()
	switch {
	case errors.Is(err, flammigration.ErrNoInstalled):
		fmt.Fprintf(b, "%s: nothing to revert\n", id)
		return nil
	case err != nil:
		return err
	}
	fmt.Fprintf(b, "%s: reverted %s (%s)\n", id, info.Id, info.Description)
	return nil
}

// status writes the migrations of the migrator, in the order they are
// applied, each with the time it was applied at, or as pending.
func (c *migrateCommand) status(
	b *strings.Builder,
	id string,
	migrator flam.IMigrator,
) error {
	infos, err := migrator.List()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Fprintf(b, "%s: no migration\n", id)
	}
	for _, info := range infos {
		state := "pending"
		if info.InstalledAt != nil {
			state = "applied at " + info.InstalledAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(b, "%s: %s %s (%s)\n", id, info.Id, state, info.Description)
	}
	return nil
}

// commandRunMessage is the message of the errors returned by Run.
const commandRunMessage = "unable to migrate"
