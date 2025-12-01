package flam

type envConfigSourceCreator struct {
	config Config
}

var _ ConfigSourceCreator = (*envConfigSourceCreator)(nil)

func newEnvConfigSourceCreator(config Config) ConfigSourceCreator {
	return &envConfigSourceCreator{
		config: config}
}

func (creator envConfigSourceCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigSourceDriverEnv
}

func (creator envConfigSourceCreator) Create(
	config Bag,
) (ConfigSource, error) {
	priority := config.Int("priority", creator.config.Int(PathConfigDefaultPriority))
	files := config.StringSlice("files", []string{})
	mappings := map[string]string{}

	for key, path := range config.Bag("mappings", Bag{}) {
		if str, ok := path.(string); ok {
			mappings[key] = str
		}
	}

	return newEnvConfigSource(
		priority,
		files,
		mappings)
}
