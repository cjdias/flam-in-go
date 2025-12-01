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

func (config factoryConfig) Get(
	path string,
	def ...any,
) Bag {
	data := config.config.aggregateBag.Get(path, def...)
	if bag, ok := data.(Bag); ok {
		return bag
	}

	return Bag{}
}
