package openapi

import (
	"cmp"
	"encoding"
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Schema is a JSON Schema of the OpenAPI document, as generated from the
// Go types of the documented values.
type Schema struct {
	// Ref is the reference to the schema of the components it stands for,
	// such as "#/components/schemas/User".
	Ref string `json:"$ref,omitempty"`

	// Type is the type of the values, or the types, such as "string" and
	// "null" for a nullable one.
	Type Types `json:"type,omitempty"`

	// Format is the format of the values, such as "date-time" or "int64".
	Format string `json:"format,omitempty"`

	// ContentEncoding is the encoding of the content of a string, such as
	// "base64".
	ContentEncoding string `json:"contentEncoding,omitempty"`

	// Properties are the schemas of the properties of an object, by name.
	Properties map[string]*Schema `json:"properties,omitempty"`

	// Required are the names of the properties an object always holds.
	Required []string `json:"required,omitempty"`

	// AdditionalProperties is the schema of the properties of an object
	// not listed in its Properties, such as the values of a map.
	AdditionalProperties *Schema `json:"additionalProperties,omitempty"`

	// Items is the schema of the items of an array.
	Items *Schema `json:"items,omitempty"`

	// MinItems is the least number of items of an array.
	MinItems *int `json:"minItems,omitempty"`

	// MaxItems is the most items of an array.
	MaxItems *int `json:"maxItems,omitempty"`

	// Minimum is the least value of a number.
	Minimum *int `json:"minimum,omitempty"`

	// AnyOf are the schemas one of which, at least, the values match, such
	// as a reference and the null type for a nullable reference.
	AnyOf []*Schema `json:"anyOf,omitempty"`
}

// Types are the types of the values of a [Schema], written as a single
// type when there is one.
type Types []string

// MarshalJSON returns the JSON of the single type, or of the list of the
// types.
func (t Types) MarshalJSON() ([]byte, error) {
	if len(t) == 1 {
		return json.Marshal(t[0])
	}
	return json.Marshal([]string(t))
}

// Types of the values of the schemas.
const (
	typeObject  = "object"
	typeArray   = "array"
	typeString  = "string"
	typeInteger = "integer"
	typeNumber  = "number"
	typeBoolean = "boolean"
	typeNull    = "null"
)

// componentsSchemasRef is the base of the references to the schemas of the
// components.
const componentsSchemasRef = "#/components/schemas/"

// Reflected types the schemas of whose values are not the ones of their
// kind.
var (
	timeType          = reflect.TypeFor[time.Time]()
	rawMessageType    = reflect.TypeFor[json.RawMessage]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()

	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// schemas generates the schemas of the Go types, the named struct types
// being added to the components, once, and referenced, so that a
// recursive type is generated once.
type schemas struct {
	components map[string]*Schema
	names      map[reflect.Type]string
}

// newSchemas returns the generator of the schemas, the names of the
// components being reserved.
func newSchemas(
	components map[string]*Schema,
) *schemas {
	return &schemas{
		components: components,
		names:      map[reflect.Type]string{},
	}
}

// of returns the schema of the values of the type of v, not nil.
func (s *schemas) of(
	v any,
) *Schema {
	return s.schema(reflect.TypeOf(v))
}

// schema returns the schema of the values of the type, as encoding/json
// writes them.
func (s *schemas) schema(
	t reflect.Type,
) *Schema {
	switch {
	case t == timeType:
		return &Schema{Type: Types{typeString}, Format: "date-time"}
	case t == rawMessageType:
		return &Schema{}
	case t.Kind() == reflect.Pointer:
		return nullable(s.schema(t.Elem()))
	case t.Implements(jsonMarshalerType) || reflect.PointerTo(t).Implements(jsonMarshalerType):
		return &Schema{}
	case t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType):
		return &Schema{Type: Types{typeString}}
	}

	switch t.Kind() {
	case reflect.Bool:
		return &Schema{Type: Types{typeBoolean}}
	case reflect.Int8, reflect.Int16, reflect.Int32:
		return &Schema{Type: Types{typeInteger}, Format: "int32"}
	case reflect.Int, reflect.Int64:
		return &Schema{Type: Types{typeInteger}, Format: "int64"}
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &Schema{Type: Types{typeInteger}, Format: "int32", Minimum: new(0)}
	case reflect.Uint, reflect.Uint64, reflect.Uintptr:
		return &Schema{Type: Types{typeInteger}, Format: "int64", Minimum: new(0)}
	case reflect.Float32:
		return &Schema{Type: Types{typeNumber}, Format: "float"}
	case reflect.Float64:
		return &Schema{Type: Types{typeNumber}, Format: "double"}
	case reflect.String:
		return &Schema{Type: Types{typeString}}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: Types{typeString}, ContentEncoding: "base64"}
		}
		return &Schema{Type: Types{typeArray}, Items: s.schema(t.Elem())}
	case reflect.Array:
		return &Schema{Type: Types{typeArray}, Items: s.schema(t.Elem()), MinItems: new(t.Len()), MaxItems: new(t.Len())}
	case reflect.Map:
		if !mapKey(t.Key()) {
			return &Schema{}
		}
		return &Schema{Type: Types{typeObject}, AdditionalProperties: s.schema(t.Elem())}
	case reflect.Struct:
		return s.object(t)
	}
	// interfaces, and the kinds encoding/json cannot write, such as the
	// channels and the functions
	return &Schema{}
}

// object returns the schema of the struct type: a reference to the schema
// of its components when named, generated when first referenced, or else
// the schema itself.
func (s *schemas) object(
	t reflect.Type,
) *Schema {
	if t.Name() == "" {
		return s.properties(t)
	}
	if name, ok := s.names[t]; ok {
		return &Schema{Ref: componentsSchemasRef + name}
	}

	name := componentName(t.Name())
	for i := 2; s.components[name] != nil; i++ {
		name = componentName(t.Name()) + strconv.Itoa(i)
	}
	s.names[t] = name
	// reserved before the properties are generated, for a recursive type
	s.components[name] = &Schema{}
	*s.components[name] = *s.properties(t)
	return &Schema{Ref: componentsSchemasRef + name}
}

// properties returns the schema of the object of the fields of the struct
// type written by encoding/json, those left out when empty, or through a
// nil embedded pointer, not being required.
func (s *schemas) properties(
	t reflect.Type,
) *Schema {
	schema := &Schema{Type: Types{typeObject}, Properties: map[string]*Schema{}}
	for _, f := range fields(t, "json") {
		if f.name == "" {
			f.name = f.field
		}
		property := s.schema(f.typ)
		if slices.Contains(f.options, "string") {
			property = quoted(f.typ, property)
		}
		schema.Properties[f.name] = property
		if !f.optional && !slices.Contains(f.options, "omitempty") && !slices.Contains(f.options, "omitzero") {
			schema.Required = append(schema.Required, f.name)
		}
	}
	return schema
}

// nullable returns the schema of the values of the schema, or null.
func nullable(
	schema *Schema,
) *Schema {
	switch {
	case schema.Ref != "":
		return &Schema{AnyOf: []*Schema{schema, {Type: Types{typeNull}}}}
	case len(schema.Type) == 0 || slices.Contains(schema.Type, typeNull):
		return schema
	}
	schema.Type = append(schema.Type, typeNull)
	return schema
}

// mapKey reports whether encoding/json writes the maps of keys of the
// type.
func mapKey(
	t reflect.Type,
) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return t.Implements(textMarshalerType)
}

// quoted returns the schema of the values of the type, of the schema,
// written by encoding/json given the "string" option of a json tag: a
// string, nullable for a pointer, for a boolean, a number or a string, or
// else the schema, as the option is then ignored.
func quoted(
	t reflect.Type,
	schema *Schema,
) *Schema {
	elem := t
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	switch elem.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.String:
		if elem != t {
			return &Schema{Type: Types{typeString, typeNull}}
		}
		return &Schema{Type: Types{typeString}}
	}
	return schema
}

// packagePath matches the path of the package of a type argument in the
// name of an instance of a generic type, such as "example.com/users." in
// "Page[example.com/users.User]".
var packagePath = regexp.MustCompile(`[^\[\],*]*/`)

// invalidNameChar matches a character not allowed in the name of a
// component.
var invalidNameChar = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// componentName returns the name of the component of the schema of the
// type of the name, without the paths of the packages of its type
// arguments, and its characters not allowed replaced by "_", such as
// "Page_users.User_" for "Page[example.com/users.User]".
func componentName(
	name string,
) string {
	return invalidNameChar.ReplaceAllString(packagePath.ReplaceAllString(name, ""), "_")
}

// field is a field of a struct type, as written or decoded.
type field struct {
	// field is the name of the Go field.
	field string
	// name is the name of the field in its tag, empty when not named.
	name string
	// options are the options of the tag, such as "omitempty".
	options []string
	// index is the index sequence of the field, its depth being its length.
	index []int
	// typ is the type of the field.
	typ reflect.Type
	// optional reports whether the field is reached through an embedded
	// pointer, left out when nil.
	optional bool
}

// fields returns the fields of the struct type written by encoding/json,
// as named by the tags of the key, or decoded by
// github.com/gorilla/schema for the "schema" key: the exported fields, not
// tagged "-", along with those of the embedded structs not named by a tag,
// a field hiding those of the same name deeper, or, at the same depth,
// those not tagged when tagged, the others of the same name at the same
// depth being left out, as encoding/json does.
func fields(
	t reflect.Type,
	key string,
) []field {
	var all []field
	collect(t, key, nil, false, map[reflect.Type]bool{}, &all)

	byName := map[string][]field{}
	for _, f := range all {
		name := cmp.Or(f.name, f.field)
		byName[name] = append(byName[name], f)
	}
	var dominant []field
	for _, f := range all {
		name := cmp.Or(f.name, f.field)
		if d, ok := dominantField(byName[name]); ok && slices.Equal(d.index, f.index) {
			dominant = append(dominant, f)
		}
	}
	return dominant
}

// collect appends the fields of the struct type, at the index, to the
// list, the fields of its embedded structs included, those of a type
// being embedded in itself being left out.
func collect(
	t reflect.Type,
	key string,
	index []int,
	optional bool,
	visiting map[reflect.Type]bool,
	all *[]field,
) {
	if visiting[t] {
		return
	}
	visiting[t] = true
	defer delete(visiting, t)

	for i := range t.NumField() {
		sf := t.Field(i)
		tag := sf.Tag.Get(key)
		if tag == "-" {
			continue
		}
		name, rest, _ := strings.Cut(tag, ",")
		var options []string
		if rest != "" {
			options = strings.Split(rest, ",")
		}
		at := append(slices.Clip(index), i)

		ft := sf.Type
		if sf.Anonymous {
			embedded, pointer := ft, false
			if embedded.Kind() == reflect.Pointer {
				embedded, pointer = embedded.Elem(), true
			}
			if name == "" && embedded.Kind() == reflect.Struct {
				collect(embedded, key, at, optional || pointer, visiting, all)
				continue
			}
			if !sf.IsExported() {
				continue
			}
		} else if !sf.IsExported() {
			continue
		}
		*all = append(*all, field{field: sf.Name, name: name, options: options, index: at, typ: ft, optional: optional})
	}
}

// dominantField returns the field of the fields of the same name hiding
// the others, if any: the only one of the least depth, or the only one
// tagged with its name among them.
func dominantField(
	fields []field,
) (field, bool) {
	depth := len(fields[0].index)
	var shallowest, tagged []field
	for _, f := range fields {
		switch {
		case len(f.index) < depth:
			depth, shallowest, tagged = len(f.index), nil, nil
			fallthrough
		case len(f.index) == depth:
			shallowest = append(shallowest, f)
			if f.name != "" {
				tagged = append(tagged, f)
			}
		}
	}
	switch {
	case len(shallowest) == 1:
		return shallowest[0], true
	case len(tagged) == 1:
		return tagged[0], true
	}
	return field{}, false
}
