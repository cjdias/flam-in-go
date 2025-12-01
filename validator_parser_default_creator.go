package flam

type defaultValidatorParserCreator struct {
	config            Config
	translatorFactory TranslatorFactory
}

var _ ValidatorParserCreator = (*defaultValidatorParserCreator)(nil)

func newDefaultValidatorParserCreator(
	config Config,
	translatorFactory TranslatorFactory,
) ValidatorParserCreator {
	return &defaultValidatorParserCreator{
		config:            config,
		translatorFactory: translatorFactory}
}

func (creator defaultValidatorParserCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ValidatorParserDriverDefault
}

func (creator defaultValidatorParserCreator) Create(
	config Bag,
) (ValidatorParser, error) {
	annotation := config.String("annotation", creator.config.String(PathValidatorDefaultAnnotation))
	translatorId := config.String("translator_id", creator.config.String(PathValidatorDefaultTranslatorId))

	switch {
	case annotation == "":
		return nil, newErrInvalidResourceConfig("defaultValidatorParser", "annotation", config)
	case translatorId == "":
		return nil, newErrInvalidResourceConfig("defaultValidatorParser", "translator_id", config)
	}

	translator, e := creator.translatorFactory.Get(translatorId)
	if e != nil {
		return nil, e
	}

	return newDefaultValidatorParser(
		annotation,
		translator), nil
}
