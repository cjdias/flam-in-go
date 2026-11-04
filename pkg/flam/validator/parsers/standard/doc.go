// Package standard is the validation parser driver of go-playground: its
// parsers describe each error of a go-playground validate by the path of
// its field from the validated struct, the id of the field, read from a
// struct tag, the code of its rule, from [TagCodes], and its message,
// translated with a translator of the
// [github.com/cjdias/flam-in-go/pkg/flam.ITranslatorFactory], named by its
// id in their translator field, or its default, such as:
//
//	type User struct {
//		Name  string `validate:"required" paramId:"1"`
//		Email string `validate:"required,email" paramId:"2"`
//	}
//
// a User missing its email failing with the ValidationError
// {ParamId: 2, ParamName: "Email", ErrorId: 104, ErrorMessage: "Email is
// a required field"}, with an English translator.
//
// The messages are the texts of go-playground for the locale of the
// translator, held by it under the tags of the rules, such as "required":
// give the translator its own texts of these keys to replace them, such
// as "required: '{0} is missing'". Without a translator, the messages are
// left as go-playground formats them. [NewParser] creates one from a
// translator taken elsewhere.
package standard
