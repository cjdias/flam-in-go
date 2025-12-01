package flam

type defaultValidatorCreator struct {
	config                         Config
	validatorParserFactory         ValidatorParserFactory
	validatorErrorConverterFactory ValidatorErrorConverterFactory
}

var _ ValidatorCreator = (*defaultValidatorCreator)(nil)

func newDefaultValidatorCreator(
	config Config,
	validatorParserFactory ValidatorParserFactory,
	validatorErrorConverterFactory ValidatorErrorConverterFactory,
) ValidatorCreator {
	return &defaultValidatorCreator{
		config:                         config,
		validatorParserFactory:         validatorParserFactory,
		validatorErrorConverterFactory: validatorErrorConverterFactory}
}

func (creator defaultValidatorCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ValidatorDriverDefault
}

func (creator defaultValidatorCreator) Create(
	config Bag,
) (Validator, error) {
	parserId := config.String("parser_id", creator.config.String(PathValidatorDefaultParserId))
	errorConverterId := config.String("error_converter_id", creator.config.String(PathValidatorDefaultErrorConverterId))

	if parserId == "" {
		return nil, newErrInvalidResourceConfig("defaultValidator", "errorConverterId", config)
	}

	parser, e := creator.validatorParserFactory.Get(parserId)
	if e != nil {
		return nil, e
	}

	var errorConverter ValidatorErrorConverter
	if errorConverterId != "" {
		errorConverter, e = creator.validatorErrorConverterFactory.Get(errorConverterId)
		if e != nil {
			return nil, e
		}
	}

	return newDefaultValidator(
		parser,
		errorConverter), nil
}
