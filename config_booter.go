package flam

type configBooter struct {
	config              Config
	configSourceFactory ConfigSourceFactory
}

func newConfigBooter(
	config Config,
	configSourceFactory ConfigSourceFactory,
) *configBooter {
	return &configBooter{
		config:              config,
		configSourceFactory: configSourceFactory}
}

func (booter *configBooter) Boot() error {
	if !booter.config.Bool(PathConfigBoot) {
		return nil
	}

	for id := range booter.config.Bag(PathConfigSources) {
		_, e := booter.configSourceFactory.Get(id)
		if e != nil {
			return e
		}
	}

	return nil
}
