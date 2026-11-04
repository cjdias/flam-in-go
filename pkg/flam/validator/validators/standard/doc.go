// Package standard is the validator driver of go-playground: its
// validators check the structs against the rules of the validate tags of
// their fields, describe the failures with a parser of the
// [github.com/cjdias/flam-in-go/pkg/flam.IValidationParserFactory], and
// convert them with a converter of the
// [github.com/cjdias/flam-in-go/pkg/flam.IValidationErrorConverterFactory],
// if any, named by their ids in their parser and converter fields, or
// their defaults. [NewValidator] creates one from a parser and a
// converter taken elsewhere.
//
//	if result := v.Validate(&user); result != nil {
//		return result // the []flam.ValidationError, or their conversion
//	}
package standard
