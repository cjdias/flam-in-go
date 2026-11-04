// Package migration is the migrate command, [Name], of the
// [github.com/cjdias/flam-in-go/pkg/flam/command] feature, running the
// [github.com/cjdias/flam-in-go/pkg/flam.IMigrator] values of the
// [github.com/cjdias/flam-in-go/pkg/flam.IMigratorFactory] of the
// container:
//
//	app migrate                 # applies every pending migration
//	app migrate up main         # of the migrator "main" only
//	app migrate down            # reverts the last applied migration
//	app migrate status          # lists the migrations, applied or pending
//
// The action, [ActionUp] unless given, runs on the migrators named after
// it, or else on every migrator configured, in id order, stopping at the
// first one failing, or when the command is interrupted.
package migration
