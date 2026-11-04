package standard_test

import (
	"fmt"

	"github.com/go-playground/locales/en"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
	validatorstandard "github.com/cjdias/flam-in-go/pkg/flam/validator/validators/standard"
)

type user struct {
	Name  string `validate:"required" paramId:"1"`
	Email string `validate:"required,email" paramId:"2"`
}

func ExampleNewValidator() {
	tr, _ := translator.NewTranslator("en", en.New())
	parser, _ := standard.NewParser("parser", "paramId", tr)
	v, err := validatorstandard.NewValidator("main", parser, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(v.Validate(&user{Name: "ann", Email: "ann@example.com"}))
	for _, e := range v.Validate(&user{Email: "ann"}).([]flam.ValidationError) {
		fmt.Println(e.ParamId, e.ParamName, e.ErrorId, e.ErrorMessage)
	}
	// Output:
	// <nil>
	// 1 Name 104 Name is a required field
	// 2 Email 63 Email must be a valid email address
}
