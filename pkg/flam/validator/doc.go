// Package validator is the validator feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IValidationParserFactory], the
// [github.com/cjdias/flam-in-go/pkg/flam.IValidationErrorConverterFactory]
// and the [github.com/cjdias/flam-in-go/pkg/flam.IValidatorFactory] of an
// application, which create the parsers, converters and validators
// described by the configuration at [ParserConfigPath],
// [ConverterConfigPath] and [ValidatorConfigPath] with the creators
// provided in the [ParserCreatorGroup], [ConverterCreatorGroup] and
// [ValidatorCreatorGroup] container groups.
//
// A validator checks a struct against the rules of the validate tags of
// its fields, with go-playground [github.com/go-playground/validator/v10]:
// its parser describes each failure as a
// [github.com/cjdias/flam-in-go/pkg/flam.ValidationError], with a message
// in the language of the translator of the parser, and its converter, if
// any, turns them into its result, such as the body of an error response.
// A validator names its parser and converter by their ids, in the
// [ConfigFieldParser] and [ConfigFieldConverter] fields, or takes their
// defaults, such as:
//
//	flam:
//	  validator:
//	    defaults:
//	      parsers:
//	        standard: {translator: en}
//	    validators:
//	      api: {driver: flam.validator.validators.driver.standard, converter: api}
//
// the parser taking its default, the one of
// [github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard],
// translating its messages into English, and the converter api being
// created by a creator of the application.
//
// The result of a validator wraps [ErrInvalidValue] when the value cannot
// be validated.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/validator/parsers],
// [github.com/cjdias/flam-in-go/pkg/flam/validator/converters] and
// [github.com/cjdias/flam-in-go/pkg/flam/validator/validators]. The
// converter driver answers the envelopes of the REST responses: as a
// converter shapes the results of an application, provide the creator of
// any other in the [ConverterCreatorGroup] group.
// The drivers import this package for its paths, groups, fields and
// errors, so this package never imports them.
package validator
