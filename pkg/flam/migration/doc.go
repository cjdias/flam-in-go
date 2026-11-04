// Package migration is the migration feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IMigratorFactory] and the
// [github.com/cjdias/flam-in-go/pkg/flam.IMigrationLoggerAdaptorFactory] of
// an application, which create the migrators and logger adaptors described
// by the configuration at [MigratorConfigPath] and [LoggerConfigPath].
//
// The application provides its migrations, built with [NewMigration] or
// [NewFileMigration], or any
// [github.com/cjdias/flam-in-go/pkg/flam.IMigration], in the
// [MigrationGroup] container group:
//
//	app.Provide(func() flam.IMigration {
//		return migration.NewFileMigration("001", "users", "main", nil, files, "sql/001_users")
//	}, dig.Group(migration.MigrationGroup))
//
// and a migrator applies those of its group, its id unless configured, to
// the database of its connection:
//
//	flam:
//	  migration:
//	    boot: true
//	    migrators:
//	      main: {connection: main}
//
// A migrator records the migrations it applied in the table
// [RecordTable], by group and id, so that several migrators may share a
// database. The logger adaptor drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/migration/loggers].
package migration
