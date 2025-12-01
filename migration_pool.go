package flam

import (
	"slices"
	"strings"

	"go.uber.org/dig"
)

type migrationPool []Migration

func newMigrationPool(args struct {
	dig.In

	Migrations []Migration `group:"flam.migration.migrations"`
}) *migrationPool {
	pool := migrationPool(args.Migrations)
	slices.SortFunc(pool, func(a, b Migration) int {
		return strings.Compare(a.Version(), b.Version())
	})

	return &pool
}

func (pool *migrationPool) Group(
	group string,
) []Migration {
	var migrations []Migration
	for _, migration := range *pool {
		if migration.Group() == group {
			migrations = append(migrations, migration)
		}
	}

	return migrations
}
