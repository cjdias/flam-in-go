package flam

type logBooter struct {
	config           Config
	logStreamFactory LogStreamFactory
}

func newLogBooter(
	config Config,
	logStreamFactory LogStreamFactory,
) *logBooter {
	return &logBooter{
		config:           config,
		logStreamFactory: logStreamFactory}
}

func (booter *logBooter) Boot() error {
	if !booter.config.Bool(PathLogBoot) {
		return nil
	}

	for id := range booter.config.Bag(PathLogStreams) {
		_, e := booter.logStreamFactory.Get(id)
		if e != nil {
			return e
		}
	}

	return nil
}
