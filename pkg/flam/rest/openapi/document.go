package openapi

import (
	"cmp"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
)

// NewDocument returns the OpenAPI document of the endpoints, described by
// the info, and served by the servers.
//
// An endpoint is documented under the path of its pattern, its wildcards
// "{name...}" written "{name}", and "{$}" left out, the host of the
// pattern being left out too, along with its method, the endpoints of a
// pattern of no method, or of a method OpenAPI does not know, being left
// out, as are those of a method and a path already documented. Its
// operation is identified by its ids, such as "s7.e12", or "e12" for the
// service 0, unless its id is 0, and holds its documentation fields:
//   - its parameters, one for each wildcard of its pattern, in the path,
//     and one for each other field of its Params struct, in the query,
//     named by their schema tags, required when tagged so, such as
//     `schema:"name,required"`, the fields of a nested struct being named
//     after it, such as "page.start", along with their schemas;
//   - its request body, of the JSON schema of the type of its Request,
//     when not nil;
//   - its successful response, of its status, or 200, holding the
//     envelope of the data of the JSON schema of the type of its Response,
//     when not nil, as written by
//     [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope], or the
//     text/event-stream content of a Streaming endpoint, while a response
//     of the 204 or 304 status holds none;
//   - its default response, the RFC 9457 problem of the [ProblemName]
//     component the errors are written as.
//
// The JSON schemas are generated from the Go types as encoding/json
// writes their values: a struct is an object of its fields named by their
// json tags, those embedded included, required unless tagged omitempty or
// omitzero, or reached through an embedded pointer, a pointer is
// nullable, a slice or an array is an array, a byte slice a base64
// string, a map an object of the additional properties of its values, a
// time.Time a date-time string, a type of a MarshalJSON method any value,
// and a type of a MarshalText method a string. A named struct is the
// component of the schemas of its name, referenced, so that a recursive
// one is generated once, the names of distinct types being numbered
// apart, such as "User2".
func NewDocument(
	info Info,
	servers []Server,
	endpoints []flam.RestEndpoint,
) *Document {
	d := &Document{
		OpenAPI: Version,
		Info:    info,
		Servers: servers,
		Paths:   map[string]PathItem{},
		Components: Components{
			Schemas:   map[string]*Schema{ProblemName: problemSchema()},
			Responses: map[string]*Response{ProblemName: problemResponse()},
		},
	}

	s := newSchemas(d.Components.Schemas)
	for _, endpoint := range endpoints {
		method, path, ok := route(endpoint.Pattern)
		if !ok {
			continue
		}
		item := d.Paths[path]
		if item == nil {
			item = PathItem{}
			d.Paths[path] = item
		}
		if item[method] == nil {
			item[method] = operation(s, endpoint)
		}
	}
	return d
}

// Document is the OpenAPI document returned by [NewDocument], written as
// its JSON.
type Document struct {
	// OpenAPI is the version of OpenAPI of the document, [Version].
	OpenAPI string `json:"openapi"`

	// Info describes the API.
	Info Info `json:"info"`

	// Servers are the servers serving the API.
	Servers []Server `json:"servers,omitempty"`

	// Paths are the operations of the endpoints, by path, then by method.
	Paths map[string]PathItem `json:"paths"`

	// Components are the schemas and the responses the operations
	// reference.
	Components Components `json:"components"`
}

// Info describes the API of a [Document].
type Info struct {
	// Title is the title of the API.
	Title string `json:"title"`

	// Version is the version of the API, such as "1.2.0".
	Version string `json:"version"`

	// Description is the description of the API.
	Description string `json:"description,omitempty"`
}

// Server is a server serving the API of a [Document].
type Server struct {
	// URL is the URL of the server, such as "https://api.example.com".
	URL string `json:"url"`
}

// PathItem are the operations of a path, by method, in lower case.
type PathItem map[string]*Operation

// Operation is the operation of an endpoint.
type Operation struct {
	// OperationId identifies the operation.
	OperationId string `json:"operationId,omitempty"`

	// Summary is the short summary of the operation.
	Summary string `json:"summary,omitempty"`

	// Description is the description of the operation.
	Description string `json:"description,omitempty"`

	// Tags group the operation with the others of the same tags.
	Tags []string `json:"tags,omitempty"`

	// Parameters are the parameters of the path and of the query.
	Parameters []*Parameter `json:"parameters,omitempty"`

	// RequestBody is the body of the requests, if any.
	RequestBody *RequestBody `json:"requestBody,omitempty"`

	// Responses are the responses, by status, or "default".
	Responses map[string]*Response `json:"responses"`

	// Deprecated reports whether the operation is deprecated.
	Deprecated bool `json:"deprecated,omitempty"`
}

// Parameter is a parameter of an [Operation].
type Parameter struct {
	// Name is the name of the parameter.
	Name string `json:"name"`

	// In is where the parameter is: "path" or "query".
	In string `json:"in"`

	// Required reports whether the parameter is required, as those of the
	// path are.
	Required bool `json:"required,omitempty"`

	// Schema is the schema of the values of the parameter.
	Schema *Schema `json:"schema"`
}

// RequestBody is the body of the requests of an [Operation].
type RequestBody struct {
	// Required reports whether the body is required.
	Required bool `json:"required,omitempty"`

	// Content is the content of the body, by content type.
	Content map[string]*MediaType `json:"content"`
}

// Response is a response of an [Operation], or the reference to the one of
// the [Components] it stands for.
type Response struct {
	// Ref is the reference to the response of the components, such as
	// "#/components/responses/Problem".
	Ref string `json:"$ref,omitempty"`

	// Description is the description of the response.
	Description string `json:"description,omitempty"`

	// Content is the content of the response, by content type, if any.
	Content map[string]*MediaType `json:"content,omitempty"`
}

// MediaType is the content of a body of a content type.
type MediaType struct {
	// Schema is the schema of the content.
	Schema *Schema `json:"schema"`
}

// Components are the schemas and the responses referenced by the
// operations of a [Document].
type Components struct {
	// Schemas are the schemas, by name.
	Schemas map[string]*Schema `json:"schemas,omitempty"`

	// Responses are the responses, by name.
	Responses map[string]*Response `json:"responses,omitempty"`
}

// Content types of the documented bodies.
const (
	contentTypeJSON        = "application/json"
	contentTypeEventStream = "text/event-stream"
)

// methods are the methods of the operations of OpenAPI.
var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// wildcard matches a wildcard of a pattern, "{name}" or "{name...}", its
// name being its first group.
var wildcard = regexp.MustCompile(`\{(\w+)(?:\.\.\.)?\}`)

// route returns the method, in lower case, and the path of the operation
// of the pattern, and whether it has one.
func route(
	pattern string,
) (string, string, bool) {
	parts := strings.Fields(pattern)
	if len(parts) != 2 {
		return "", "", false
	}
	method := strings.ToLower(parts[0])
	start := strings.Index(parts[1], "/")
	if !slices.Contains(methods, method) || start < 0 {
		return "", "", false
	}
	path := strings.ReplaceAll(parts[1][start:], "{$}", "")
	return method, wildcard.ReplaceAllString(path, "{$1}"), true
}

// operation returns the operation of the endpoint, its schemas generated
// by s.
func operation(
	s *schemas,
	endpoint flam.RestEndpoint,
) *Operation {
	op := &Operation{
		OperationId: operationId(endpoint),
		Summary:     endpoint.Summary,
		Description: endpoint.Description,
		Tags:        endpoint.Tags,
		Parameters:  parameters(s, endpoint),
		Responses:   map[string]*Response{"default": {Ref: componentsResponsesRef + ProblemName}},
		Deprecated:  endpoint.Deprecated,
	}

	if endpoint.Request != nil {
		op.RequestBody = &RequestBody{
			Required: true,
			Content:  map[string]*MediaType{contentTypeJSON: {Schema: s.of(endpoint.Request)}},
		}
	}

	status := cmp.Or(endpoint.Status, http.StatusOK)
	response := &Response{Description: cmp.Or(http.StatusText(status), "Success")}
	switch {
	case status == http.StatusNoContent || status == http.StatusNotModified:
	case endpoint.Streaming:
		response.Content = map[string]*MediaType{contentTypeEventStream: {Schema: &Schema{Type: Types{typeString}}}}
	case endpoint.Response != nil:
		response.Content = map[string]*MediaType{contentTypeJSON: {Schema: &Schema{
			Type:       Types{typeObject},
			Properties: map[string]*Schema{"data": s.of(endpoint.Response)},
			Required:   []string{"data"},
		}}}
	}
	op.Responses[strconv.Itoa(status)] = response
	return op
}

// operationId returns the id of the operation of the endpoint, made of the
// ids of its service, unless 0, and of the endpoint, or empty when the id
// of the endpoint is 0.
func operationId(
	endpoint flam.RestEndpoint,
) string {
	switch {
	case endpoint.Id == 0:
		return ""
	case endpoint.ServiceId == 0:
		return "e" + strconv.Itoa(endpoint.Id)
	}
	return "s" + strconv.Itoa(endpoint.ServiceId) + ".e" + strconv.Itoa(endpoint.Id)
}

// parameters returns the parameters of the endpoint: those of the
// wildcards of its pattern, in the path, then those of the other fields of
// its Params, in the query.
func parameters(
	s *schemas,
	endpoint flam.RestEndpoint,
) []*Parameter {
	var fields []*Parameter
	if endpoint.Params != nil {
		t := reflect.TypeOf(endpoint.Params)
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			fields = params(s, t, "")
		}
	}

	var path, query []*Parameter
	names := map[string]bool{}
	for _, match := range wildcard.FindAllStringSubmatch(endpoint.Pattern, -1) {
		names[match[1]] = true
		parameter := &Parameter{Name: match[1], Schema: &Schema{Type: Types{typeString}}}
		if i := slices.IndexFunc(fields, func(p *Parameter) bool { return p.Name == match[1] }); i >= 0 {
			parameter = fields[i]
		}
		parameter.In, parameter.Required = "path", true
		path = append(path, parameter)
	}
	for _, parameter := range fields {
		if !names[parameter.Name] {
			parameter.In = "query"
			query = append(query, parameter)
		}
	}
	return append(path, query...)
}

// params returns the parameters of the fields of the struct type, as
// decoded by github.com/gorilla/schema, named after the prefix.
func params(
	s *schemas,
	t reflect.Type,
	prefix string,
) []*Parameter {
	var parameters []*Parameter
	for _, f := range fields(t, "schema") {
		name := prefix + cmp.Or(f.name, f.field)
		ft := f.typ
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct && !reflect.PointerTo(ft).Implements(textUnmarshalerType) {
			parameters = append(parameters, params(s, ft, name+".")...)
			continue
		}
		parameters = append(parameters, &Parameter{
			Name:     name,
			Required: slices.Contains(f.options, "required"),
			Schema:   s.schema(ft),
		})
	}
	return parameters
}

// componentsResponsesRef is the base of the references to the responses
// of the components.
const componentsResponsesRef = "#/components/responses/"

// problemSchema returns the schema of the RFC 9457 problems the envelope
// errors are written as, along with their extensions.
func problemSchema() *Schema {
	uri := func() *Schema { return &Schema{Type: Types{typeString}, Format: "uri-reference"} }
	str := func() *Schema { return &Schema{Type: Types{typeString}} }
	status := func() *Schema { return &Schema{Type: Types{typeInteger}, Format: "int32"} }
	return &Schema{
		Type: Types{typeObject},
		Properties: map[string]*Schema{
			"type":     uri(),
			"title":    str(),
			"status":   status(),
			"detail":   str(),
			"instance": uri(),
			"id":       str(),
			"errors": {Type: Types{typeArray}, Items: &Schema{
				Type: Types{typeObject},
				Properties: map[string]*Schema{
					"status":  status(),
					"id":      str(),
					"context": {Type: Types{typeObject}, AdditionalProperties: &Schema{}},
				},
				Required: []string{"status", "id", "context"},
			}},
		},
		Required:             []string{"type", "title", "status"},
		AdditionalProperties: &Schema{},
	}
}

// problemResponse returns the response of the problems of the
// ProblemName schema.
func problemResponse() *Response {
	return &Response{
		Description: "Error",
		Content: map[string]*MediaType{envelope.ContentTypeProblem: {
			Schema: &Schema{Ref: componentsSchemasRef + ProblemName},
		}},
	}
}
