package migration

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newBooter returns the booter applying the pending migrations of the
// configured migrators. It is a provider registered by the bundle returned
// from [NewBundle].
func newBooter(
	config flam.IConfig,
	migrators flam.IMigratorFactory,
) *booter {
	return &booter{
		config:    config,
		migrators: migrators,
	}
}

// booter applies, when the application boots, the pending migrations of
// the migrators configured at [MigratorConfigPath].
type booter struct {
	config    flam.IConfig
	migrators flam.IMigratorFactory
}

// Boot applies, in migrator id order, every pending migration of each
// migrator configured at [MigratorConfigPath], when [BootConfigPath]
// holds true. It stops at the first migrator failing to be created or to
// apply a migration, leaving applied the migrations applied before it.
func (b *booter) Boot() error {
	if !b.config.Bool(BootConfigPath, defaultBoot) {
		return nil
	}

	for _, id := range b.migrators.Available() {
		migrator, err := b.migrators.Get(id)
		if err != nil {
			return newErrBoot(booterBootMessage, id, err)
		}
		if _, err := migrator.UpAll(); err != nil {
			return newErrBoot(booterBootMessage, id, err)
		}
	}
	return nil
}

// booterBootMessage is the message of the errors returned by Boot.
const booterBootMessage = "unable to boot migrator"
