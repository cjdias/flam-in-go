package standard

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-playground/locales"
	ut "github.com/go-playground/universal-translator"
	govalidator "github.com/go-playground/validator/v10"
	detranslations "github.com/go-playground/validator/v10/translations/de"
	entranslations "github.com/go-playground/validator/v10/translations/en"
	estranslations "github.com/go-playground/validator/v10/translations/es"
	frtranslations "github.com/go-playground/validator/v10/translations/fr"
	pttranslations "github.com/go-playground/validator/v10/translations/pt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewParser returns the [flam.IValidationParser] of the id reading the
// ids of the fields from their struct tag named by the annotation, and
// translating the messages with the translator, or leaving them as
// go-playground formats them when it is nil. Its codes are those of
// [TagCodes].
//
// The messages are the default texts of go-playground for the locale of
// the translator, en, pt, es, fr or de, NewParser failing with ErrLocale
// for another. Register adds them to the translator under the tags of
// their rules, such as "required", but those the translator holds
// already, so that the texts of the translator replace them. The parser
// may be used from several goroutines.
func NewParser(
	id string,
	annotation string,
	translator flam.ITranslator,
) (flam.IValidationParser, error) {
	p := &parser{id: id, annotation: annotation, codes: TagCodes()}
	if value.IsNil(translator) {
		return p, nil
	}

	register, ok := registrations[translator.Locale()]
	if !ok {
		return nil, newErrLocale(parserNewMessage, id, translator.Locale())
	}
	p.translations = &translations{ITranslator: translator}
	p.register = register
	return p, nil
}

// parser is the [flam.IValidationParser] returned by [NewParser], whose
// messages are translated by translations, unless nil, with the texts
// added by register, while mu guards its codes.
type parser struct {
	id           string
	annotation   string
	translations *translations
	register     registration

	mu    sync.RWMutex
	codes map[string]int
}

// compile time check that *parser implements IValidationParser
var _ flam.IValidationParser = (*parser)(nil)

func (p *parser) Id() string {
	return p.id
}

func (p *parser) Register(
	validate *govalidator.Validate,
) error {
	if validate == nil {
		return newErrNil(ErrNilValidate, parserRegisterMessage, p.id)
	}
	if p.translations == nil {
		return nil
	}

	if err := p.register(validate, p.translations); err != nil {
		return newErrRegister(parserRegisterMessage, p.id, p.translations.Locale(), err)
	}
	return nil
}

func (p *parser) AddTagCode(
	tag string,
	code int,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.codes[tag] = code
}

// Parse returns the description of each of the errors, the path of its
// field, its id and the code of its rule, or else of the rule it is an
// alias of, and its message. The ids are read from the type of the value,
// following the path of the field, 0 when not found.
func (p *parser) Parse(
	value any,
	errs govalidator.ValidationErrors,
) []flam.ValidationError {
	p.mu.RLock()
	defer p.mu.RUnlock()

	parsed := make([]flam.ValidationError, 0, len(errs))
	for _, fe := range errs {
		name, tag := field(value, fe.StructNamespace())
		parsed = append(parsed, flam.ValidationError{
			ParamId:      p.paramId(tag),
			ParamName:    name,
			ErrorId:      p.code(fe),
			ErrorMessage: p.message(fe),
		})
	}
	return parsed
}

// Messages of the errors returned by NewParser and the parser.
const (
	parserNewMessage      = "unable to create standard validation parser"
	parserRegisterMessage = "unable to register standard validation parser messages"
)

// registration registers the messages of a locale on a validate, in a
// translator.
type registration func(validate *govalidator.Validate, translator ut.Translator) error

// registrations are the registrations of the messages of go-playground,
// by locale.
var registrations = map[string]registration{
	"de": detranslations.RegisterDefaultTranslations,
	"en": entranslations.RegisterDefaultTranslations,
	"es": estranslations.RegisterDefaultTranslations,
	"fr": frtranslations.RegisterDefaultTranslations,
	"pt": pttranslations.RegisterDefaultTranslations,
}

// translations is the translator of the messages of a parser, adding the
// texts and cardinal plural texts of go-playground, the only ones it adds,
// to the translator of the parser but those it holds already, as
// go-playground refuses to add a text twice, so that the texts of the
// translator replace them, and so that the messages can be registered on
// several validates. Being a distinct value, it keeps the registrations
// of the parser apart from those of another parser of the same
// translator.
type translations struct {
	flam.ITranslator
}

func (t *translations) Add(
	key any,
	text string,
	override bool,
) error {
	return ignoreConflict(t.ITranslator.Add(key, text, override))
}

func (t *translations) AddCardinal(
	key any,
	text string,
	rule locales.PluralRule,
	override bool,
) error {
	return ignoreConflict(t.ITranslator.AddCardinal(key, text, rule, override))
}

// ignoreConflict returns the error, but nil for the error of a text added
// twice.
func ignoreConflict(
	err error,
) error {
	if conflict := (*ut.ErrConflictingTranslation)(nil); errors.As(err, &conflict) {
		return nil
	}
	return err
}

// paramId returns the id held by the annotation of the tag, or 0 when
// missing or not an integer.
func (p *parser) paramId(
	tag reflect.StructTag,
) int {
	id, err := strconv.Atoi(tag.Get(p.annotation))
	if err != nil {
		return 0
	}
	return id
}

// code returns the code of the rule of the error, or else of the rule it
// is an alias of, or 0.
func (p *parser) code(
	fe govalidator.FieldError,
) int {
	if code, ok := p.codes[fe.Tag()]; ok {
		return code
	}
	return p.codes[fe.ActualTag()]
}

// message returns the message of the error, translated when the parser
// has a translator.
func (p *parser) message(
	fe govalidator.FieldError,
) string {
	if p.translations == nil {
		return fe.Error()
	}
	return fe.Translate(p.translations)
}

// field returns the path of the field of the struct namespace, such as
// "User.Address.Street", from the struct, "Address.Street", and the
// struct tag of the field, found in the type of the value, or an empty
// tag when not found.
func field(
	value any,
	namespace string,
) (string, reflect.StructTag) {
	parts := strings.Split(namespace, ".")[1:]
	name := strings.Join(parts, ".")

	t := reflect.TypeOf(value)
	tag := reflect.StructTag("")
	for _, part := range parts {
		fieldName, indexes := part, 0
		if i := strings.IndexByte(part, '['); i >= 0 {
			fieldName, indexes = part[:i], strings.Count(part[i:], "[")
		}

		st := elem(t, 0)
		if st == nil || st.Kind() != reflect.Struct {
			return name, ""
		}
		f, ok := st.FieldByName(fieldName)
		if !ok {
			return name, ""
		}
		tag, t = f.Tag, elem(f.Type, indexes)
	}
	return name, tag
}

// elem returns the type t, past its pointers and the elements of the
// given number of slices, arrays or maps, or nil when t has fewer.
func elem(
	t reflect.Type,
	indexes int,
) reflect.Type {
	for t != nil {
		switch t.Kind() {
		case reflect.Pointer:
			t = t.Elem()
		case reflect.Slice, reflect.Array, reflect.Map:
			if indexes == 0 {
				return t
			}
			t, indexes = t.Elem(), indexes-1
		default:
			if indexes > 0 {
				return nil
			}
			return t
		}
	}
	return nil
}
