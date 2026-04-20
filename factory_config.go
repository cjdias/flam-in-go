package flam

type FactoryConfig interface {
	Get(path string, def ...any) Bag
}

type factoryConfig struct {
	config *config
}

var _ FactoryConfig = (*factoryConfig)(nil)

func newFactoryConfig(
	config *config,
) FactoryConfig {
	return &factoryConfig{
		config: config}
}

func (config *factoryConfig) Get(
	path string,
	def ...any,
) Bag {
	if config.config == nil {
		if len(def) > 0 {
			if bag, ok := asBag(def[0]); ok {
				return bag
			}
		}
		return Bag{}
	}

	data := config.config.aggregateBag.Get(path, def...)
	if bag, ok := asBag(data); ok {
		return bag
	}

	// Use default if data is not a Bag
	if len(def) > 0 {
		if bag, ok := asBag(def[0]); ok {
			return bag
		}
	}

	return Bag{}
}
