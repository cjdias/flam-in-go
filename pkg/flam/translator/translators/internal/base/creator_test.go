package base

import (
	"errors"
	"testing"

	"github.com/go-playground/locales"

	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/translator"
)

// The test lives next to the package, unlike the other tests of flam,
// since an internal package cannot be imported from the tests directory.
// The drivers, tested from there, never return a nil locale.

func Test_Creator_NilLocale(t *testing.T) {
	c := NewCreator("driver", func() locales.Translator { return nil })
	bag := container.Bag{factory.ConfigFieldDriver: "driver", factory.ConfigFieldId: "main"}
	if !c.Accept(bag) {
		t.Fatal("Accept() = false, want true")
	}
	tr, err := c.Create(bag)
	if !errors.Is(err, translator.ErrNilLocale) {
		t.Fatalf("Create() = %v, want translator.ErrNilLocale", err)
	}
	if tr != nil {
		t.Fatalf("Create() = %v, want nil", tr)
	}
}
