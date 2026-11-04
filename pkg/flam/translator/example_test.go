package translator_test

import (
	"fmt"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"

	"github.com/cjdias/flam-in-go/pkg/flam/translator"
)

func ExampleNewTranslator() {
	tr, err := translator.NewTranslator("main", en.New())
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = tr.Add("welcome", "Welcome, {0}!", false)
	_ = tr.AddCardinal("days", "{0} day", locales.PluralRuleOne, false)
	_ = tr.AddCardinal("days", "{0} days", locales.PluralRuleOther, false)

	welcome, _ := tr.T("welcome", "Ann")
	one, _ := tr.C("days", 1, 0, "1")
	many, _ := tr.C("days", 3, 0, "3")
	fmt.Println(welcome)
	fmt.Println(one)
	fmt.Println(many)
	// Output:
	// Welcome, Ann!
	// 1 day
	// 3 days
}
