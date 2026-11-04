// Package english is the translator driver of the English language, with
// the "en" locale of [github.com/go-playground/locales/en]. Its bundle
// configures the translator of id [Id], holding no text: give it texts
// with the
// [github.com/cjdias/flam-in-go/pkg/flam/translator.ConfigFieldTranslations]
// field of its configuration, such as:
//
//	flam:
//	  translator:
//	    translators:
//	      en:
//	        translations:
//	          welcome: "..."
package english
