// Package translator is the translator feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.ITranslatorFactory] of an
// application, which creates the translators described by the
// configuration at [ConfigPath] with the creators provided in the
// [CreatorGroup] container group. [NewTranslator] returns the translator
// of a go-playground locale, for the creators.
//
// The drivers, one per language, live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/translator/translators], each
// configuring a translator of its locale by default, such as "en". A
// translator starts with the texts of its [ConfigFieldTranslations] field,
// such as:
//
//	flam:
//	  translator:
//	    translators:
//	      en:
//	        driver: flam.translator.translators.driver.english
//	        translations:
//	          welcome: "Welcome, {0}!"
//	          days_left:
//	            cardinal: {one: "{0} day left", other: "{0} days left"}
//	          place:
//	            ordinal: {one: "{0}st", two: "{0}nd", few: "{0}rd", other: "{0}th"}
//	          errors:
//	            required: "{0} is required"
//
// A string is the text of its key, as added by Add, such as "welcome". A
// bag holding a [TranslationFieldCardinal], [TranslationFieldOrdinal] or
// [TranslationFieldRange] field is the plural text of its key, as added
// by AddCardinal, AddOrdinal or AddRange, each holding its texts per
// plural rule name, zero, one, two, few, many or other, and needing a text
// for every plural rule of the locale. Any other bag holds the texts of
// the keys it prefixes with its key and a dot, such as "errors.required".
// The keys are strings, translated with:
//
//	t.T("welcome", "Ann")
//	t.C("days_left", 2, 0, "2")
//
// The creators fail with an error wrapping [ErrInvalidField] when the
// field is not a bag, and [ErrTranslation] for an invalid text.
//
// The drivers import this package for its paths, groups, fields and
// errors, so this package never imports them.
package translator
