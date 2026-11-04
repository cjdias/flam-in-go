package german

import (
	"github.com/go-playground/locales"
	"github.com/go-playground/locales/de"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/internal/base"
)

// newCreator returns the [flam.ITranslatorCreator] of the configurations
// with the [Driver] driver. It is the provider registered by the bundle
// returned from [NewBundle].
func newCreator() flam.ITranslatorCreator {
	return base.NewCreator(Driver, func() locales.Translator { return de.New() })
}
