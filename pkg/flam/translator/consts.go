package translator

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.translator"

// Configuration path and container group of the translators.
const (
	// ConfigPath is the configuration path holding one translator
	// configuration per translator id, as read by the translator factory.
	ConfigPath = "flam.translator.translators"

	// CreatorGroup is the container group the translator driver bundles
	// provide their [flam.ITranslatorCreator] in, collected by the
	// translator factory.
	CreatorGroup = "flam.translator.translators.creator"
)

// Fields of a translator configuration shared by the drivers of
// [github.com/cjdias/flam-in-go/pkg/flam/translator/translators].
const (
	// ConfigFieldTranslations is the field holding the texts the
	// translator starts with, a bag of keys, as described in the package
	// documentation.
	ConfigFieldTranslations = "translations"
)

// Fields of a plural text of the [ConfigFieldTranslations] field, each
// holding its texts per plural rule name: zero, one, two, few, many or
// other.
const (
	// TranslationFieldCardinal holds the texts of the cardinal plural
	// rules, such as "1 day" and "2 days".
	TranslationFieldCardinal = "cardinal"

	// TranslationFieldOrdinal holds the texts of the ordinal plural rules,
	// such as "1st" and "2nd".
	TranslationFieldOrdinal = "ordinal"

	// TranslationFieldRange holds the texts of the range plural rules,
	// such as "1-2 days".
	TranslationFieldRange = "range"
)

// Context fields of the [*flam.Error] values returned by the translators
// and their drivers.
const (
	// ErrorFieldID holds the id of a translator.
	ErrorFieldID = "id"

	// ErrorFieldLocale holds the locale of a translator, such as "en".
	ErrorFieldLocale = "locale"

	// ErrorFieldKey holds the key of a text.
	ErrorFieldKey = "key"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)
