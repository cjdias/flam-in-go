package flam

type migratorBooter struct {
	config          Config
	migratorFactory MigratorFactory
}

func newMigratorBooter(
	config Config,
	migratorFactory MigratorFactory,
) *migratorBooter {
	return &migratorBooter{
		config:          config,
		migratorFactory: migratorFactory}
}

func (booter *migratorBooter) Boot() error {
	if !booter.config.Bool(PathMigratorBoot) {
		return nil
	}

	for _, id := range booter.migratorFactory.Available() {
		migrator, e := booter.migratorFactory.Get(id)
		if e != nil {
			return e
		}

		if e := migrator.UpAll(); e != nil {
			return e
		}
	}

	return nil
}
