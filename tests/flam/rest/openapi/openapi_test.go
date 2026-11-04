package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/openapi"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// equalJSON fails the test unless the JSON of got is the JSON of want.
func equalJSON(t *testing.T, name string, got any, want string) {
	t.Helper()
	data, err := json.Marshal(got)
	testutil.NoError(t, err)
	var g, w any
	testutil.NoError(t, json.Unmarshal(data, &g))
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("%s: invalid want: %v", name, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("%s = %s, want %s", name, data, want)
	}
}

// document returns the document of the endpoints, of no info.
func document(endpoints ...flam.RestEndpoint) *openapi.Document {
	return openapi.NewDocument(openapi.Info{Title: "T", Version: "1"}, nil, endpoints)
}

// schemaOf returns the document of a GET endpoint of the response, along
// with the schema of its data.
func schemaOf(response any) (*openapi.Document, *openapi.Schema) {
	d := document(flam.RestEndpoint{Pattern: "GET /x", Response: response})
	return d, d.Paths["/x"]["get"].Responses["200"].Content["application/json"].Schema.Properties["data"]
}

// jsonMarshaler is written by its MarshalJSON method.
type jsonMarshaler struct{}

func (jsonMarshaler) MarshalJSON() ([]byte, error) { return []byte(`"x"`), nil }

// textMarshaler is written by its MarshalText method.
type textMarshaler struct{}

func (*textMarshaler) MarshalText() ([]byte, error) { return []byte("x"), nil }

// Label is a string type, embedded.
type Label string

// label is an unexported string type, embedded.
type label string

// inner is an unexported struct, embedded.
type inner struct {
	Inner string `json:"inner"`
}

// Extra is a struct embedded as a pointer.
type Extra struct {
	Extra int `json:"extra"`
}

// Named is a struct embedded with a json name.
type Named struct {
	N int `json:"n"`
}

// Loop is a struct embedding itself.
type Loop struct {
	*Loop
	V int `json:"v"`
}

// Node is a recursive struct.
type Node struct {
	Name     string `json:"name"`
	Children []Node `json:"children"`
	Parent   *Node  `json:"parent"`
}

// Left and Right are embedded structs of fields of the same names.
type (
	Left struct {
		Same   string `json:"same"`
		Plain  string
		Hidden string `json:"hidden"`
		Other  string `json:"Twin"`
		Shared bool
	}
	Right struct {
		Same   string
		Plain  string
		Twin   int
		Shared bool
	}
)

// Page is a generic struct.
type Page[T any] struct {
	Items []T `json:"items"`
}

// Problem is a struct of the name of the problem schema.
type Problem struct {
	Code string `json:"code"`
}

func Test_NewDocument_Schemas(t *testing.T) {
	t.Run("should generate the schemas of the kinds", func(t *testing.T) {
		var anyValue any
		for name, tc := range map[string]struct {
			value any
			want  string
		}{
			"bool":            {true, `{"type":"boolean"}`},
			"int8":            {int8(1), `{"type":"integer","format":"int32"}`},
			"int16":           {int16(1), `{"type":"integer","format":"int32"}`},
			"int32":           {int32(1), `{"type":"integer","format":"int32"}`},
			"int":             {1, `{"type":"integer","format":"int64"}`},
			"int64":           {int64(1), `{"type":"integer","format":"int64"}`},
			"uint8":           {uint8(1), `{"type":"integer","format":"int32","minimum":0}`},
			"uint16":          {uint16(1), `{"type":"integer","format":"int32","minimum":0}`},
			"uint32":          {uint32(1), `{"type":"integer","format":"int32","minimum":0}`},
			"uint":            {uint(1), `{"type":"integer","format":"int64","minimum":0}`},
			"uint64":          {uint64(1), `{"type":"integer","format":"int64","minimum":0}`},
			"uintptr":         {uintptr(1), `{"type":"integer","format":"int64","minimum":0}`},
			"float32":         {float32(1), `{"type":"number","format":"float"}`},
			"float64":         {1.5, `{"type":"number","format":"double"}`},
			"string":          {"s", `{"type":"string"}`},
			"bytes":           {[]byte("b"), `{"type":"string","contentEncoding":"base64"}`},
			"slice":           {[]string{}, `{"type":"array","items":{"type":"string"}}`},
			"array":           {[2]bool{}, `{"type":"array","items":{"type":"boolean"},"minItems":2,"maxItems":2}`},
			"map":             {map[string]int{}, `{"type":"object","additionalProperties":{"type":"integer","format":"int64"}}`},
			"int key map":     {map[int]bool{}, `{"type":"object","additionalProperties":{"type":"boolean"}}`},
			"text key map":    {map[netip.Addr]bool{}, `{"type":"object","additionalProperties":{"type":"boolean"}}`},
			"invalid key map": {map[[2]int]bool{}, `{}`},
			"time":            {time.Time{}, `{"type":"string","format":"date-time"}`},
			"raw message":     {json.RawMessage(`1`), `{}`},
			"json marshaler":  {jsonMarshaler{}, `{}`},
			"text marshaler":  {textMarshaler{}, `{"type":"string"}`},
			"text pointer":    {&netip.Addr{}, `{"type":["string","null"]}`},
			"any slice":       {[]any{}, `{"type":"array","items":{}}`},
			"any pointer":     {&anyValue, `{}`},
			"pointer pointer": {new(new(1)), `{"type":["integer","null"],"format":"int64"}`},
			"channel":         {make(chan int), `{}`},
			"function":        {func() {}, `{}`},
			"anonymous":       {struct{ A int }{}, `{"type":"object","properties":{"A":{"type":"integer","format":"int64"}},"required":["A"]}`},
			"empty anonymous": {struct{}{}, `{"type":"object"}`},
		} {
			d, schema := schemaOf(tc.value)
			equalJSON(t, name, schema, tc.want)
			testutil.Equal(t, name+" components", len(d.Components.Schemas), 1)
		}
	})

	t.Run("should generate the schema of the fields of a struct", func(t *testing.T) {
		type fields struct {
			Label
			label
			inner
			*Extra
			Named   `json:"named"`
			Id      int64 `json:"id"`
			Name    string
			Age     *int              `json:"age"`
			Tags    []string          `json:"tags,omitempty"`
			Meta    map[string]any    `json:"meta,omitzero"`
			Inline  struct{ X bool }  `json:"inline"`
			Count   int               `json:"count,string"`
			Ratio   *float64          `json:",string"`
			Skipped string            `json:"-"`
			Created time.Time         `json:"created"`
			Opt     *json.RawMessage  `json:"opt,omitempty"`
			Labels  map[string]string `json:"labels"`
			private int
		}
		var f fields
		_, _ = f.label, f.private
		d, schema := schemaOf(f)
		equalJSON(t, "ref", schema, `{"$ref":"#/components/schemas/fields"}`)
		equalJSON(t, "schema", d.Components.Schemas["fields"], `{
			"type":"object",
			"properties":{
				"Label":{"type":"string"},
				"inner":{"type":"string"},
				"extra":{"type":"integer","format":"int64"},
				"named":{"$ref":"#/components/schemas/Named"},
				"id":{"type":"integer","format":"int64"},
				"Name":{"type":"string"},
				"age":{"type":["integer","null"],"format":"int64"},
				"tags":{"type":"array","items":{"type":"string"}},
				"meta":{"type":"object","additionalProperties":{}},
				"inline":{"type":"object","properties":{"X":{"type":"boolean"}},"required":["X"]},
				"count":{"type":"string"},
				"Ratio":{"type":["string","null"]},
				"created":{"type":"string","format":"date-time"},
				"opt":{},
				"labels":{"type":"object","additionalProperties":{"type":"string"}}
			},
			"required":["Label","inner","named","id","Name","age","inline","count","Ratio","created","labels"]
		}`)
		equalJSON(t, "named", d.Components.Schemas["Named"], `{"type":"object","properties":{"n":{"type":"integer","format":"int64"}},"required":["n"]}`)
	})

	t.Run("should ignore the string option of a field of another kind, and name a field \"-\"", func(t *testing.T) {
		// built at run time, as the linters reject these tags
		value := reflect.New(reflect.StructOf([]reflect.StructField{
			{Name: "List", Type: reflect.TypeFor[[]int](), Tag: `json:"list,string"`},
			{Name: "Dash", Type: reflect.TypeFor[string](), Tag: `json:"-,"`},
		})).Elem().Interface()
		_, schema := schemaOf(value)
		equalJSON(t, "schema", schema, `{
			"type":"object",
			"properties":{
				"list":{"type":"array","items":{"type":"integer","format":"int64"}},
				"-":{"type":"string"}
			},
			"required":["list","-"]
		}`)
	})

	t.Run("should hide the fields of the same name as encoding/json does", func(t *testing.T) {
		type hiding struct {
			Plain string
			Left
			Right
			Same string `json:"Same"`
		}
		d, _ := schemaOf(hiding{})
		equalJSON(t, "schema", d.Components.Schemas["hiding"], `{
			"type":"object",
			"properties":{
				"Plain":{"type":"string"},
				"same":{"type":"string"},
				"hidden":{"type":"string"},
				"Twin":{"type":"string"},
				"Same":{"type":"string"}
			},
			"required":["Plain","same","hidden","Twin","Same"]
		}`)

		type deeper struct {
			Left
			Same string `json:"same"`
		}
		d, _ = schemaOf(deeper{})
		equalJSON(t, "deeper", d.Components.Schemas["deeper"], `{
			"type":"object",
			"properties":{
				"same":{"type":"string"},
				"Plain":{"type":"string"},
				"hidden":{"type":"string"},
				"Twin":{"type":"string"},
				"Shared":{"type":"boolean"}
			},
			"required":["Plain","hidden","Twin","Shared","same"]
		}`)
	})

	t.Run("should reference the recursive structs", func(t *testing.T) {
		d, schema := schemaOf([]Node{})
		equalJSON(t, "schema", schema, `{"type":"array","items":{"$ref":"#/components/schemas/Node"}}`)
		equalJSON(t, "node", d.Components.Schemas["Node"], `{
			"type":"object",
			"properties":{
				"name":{"type":"string"},
				"children":{"type":"array","items":{"$ref":"#/components/schemas/Node"}},
				"parent":{"anyOf":[{"$ref":"#/components/schemas/Node"},{"type":"null"}]}
			},
			"required":["name","children","parent"]
		}`)

		d, _ = schemaOf(Loop{})
		equalJSON(t, "loop", d.Components.Schemas["Loop"], `{"type":"object","properties":{"v":{"type":"integer","format":"int64"}},"required":["v"]}`)
	})

	t.Run("should name the components apart", func(t *testing.T) {
		type Node struct {
			Other int `json:"other"`
		}
		type pair struct {
			A  Node            `json:"a"`
			B  *Node           `json:"b"`
			C  []Node          `json:"c"`
			D  Node            `json:"d"`
			E  Problem         `json:"e"`
			P  Page[Problem]   `json:"p"`
			Q  *Page[*Problem] `json:"q"`
			_N Node
		}
		var p pair
		_ = p._N
		d, _ := schemaOf(p)
		equalJSON(t, "pair", d.Components.Schemas["pair"], `{
			"type":"object",
			"properties":{
				"a":{"$ref":"#/components/schemas/Node"},
				"b":{"anyOf":[{"$ref":"#/components/schemas/Node"},{"type":"null"}]},
				"c":{"type":"array","items":{"$ref":"#/components/schemas/Node"}},
				"d":{"$ref":"#/components/schemas/Node"},
				"e":{"$ref":"#/components/schemas/Problem2"},
				"p":{"$ref":"#/components/schemas/Page_openapi.Problem_"},
				"q":{"anyOf":[{"$ref":"#/components/schemas/Page__openapi.Problem_"},{"type":"null"}]}
			},
			"required":["a","b","c","d","e","p","q"]
		}`)
		equalJSON(t, "problem", d.Components.Schemas["Problem2"], `{"type":"object","properties":{"code":{"type":"string"}},"required":["code"]}`)
		equalJSON(t, "page", d.Components.Schemas["Page_openapi.Problem_"], `{
			"type":"object",
			"properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/Problem2"}}},
			"required":["items"]
		}`)

		d = document(
			flam.RestEndpoint{Pattern: "GET /a", Response: Node{}},
			flam.RestEndpoint{Pattern: "GET /b", Response: []Node{}},
		)
		local := reflect.TypeFor[Node]().Name()
		testutil.Equal(t, "local name", local, "Node")
		equalJSON(t, "first", d.Paths["/a"]["get"].Responses["200"].Content["application/json"].Schema.Properties["data"],
			`{"$ref":"#/components/schemas/Node"}`)
		equalJSON(t, "second", d.Paths["/b"]["get"].Responses["200"].Content["application/json"].Schema.Properties["data"],
			`{"type":"array","items":{"$ref":"#/components/schemas/Node"}}`)

		d = document(
			flam.RestEndpoint{Pattern: "GET /a", Response: Node{}},
			flam.RestEndpoint{Pattern: "GET /b", Response: []openapiNode{}},
		)
		equalJSON(t, "numbered", d.Paths["/b"]["get"].Responses["200"].Content["application/json"].Schema.Properties["data"],
			`{"type":"array","items":{"$ref":"#/components/schemas/Node2"}}`)
	})
}

// openapiNode is the Node of the package, apart from the Node of a test.
type openapiNode = Node

func Test_NewDocument(t *testing.T) {
	t.Run("should describe the API and the problems", func(t *testing.T) {
		d := openapi.NewDocument(openapi.Info{Title: "Users", Version: "1.2.0", Description: "The users"},
			[]openapi.Server{{URL: "https://api.example.com"}}, nil)
		equalJSON(t, "document", d, `{
			"openapi":"3.1.0",
			"info":{"title":"Users","version":"1.2.0","description":"The users"},
			"servers":[{"url":"https://api.example.com"}],
			"paths":{},
			"components":{
				"schemas":{"Problem":{
					"type":"object",
					"properties":{
						"type":{"type":"string","format":"uri-reference"},
						"title":{"type":"string"},
						"status":{"type":"integer","format":"int32"},
						"detail":{"type":"string"},
						"instance":{"type":"string","format":"uri-reference"},
						"id":{"type":"string"},
						"errors":{"type":"array","items":{
							"type":"object",
							"properties":{
								"status":{"type":"integer","format":"int32"},
								"id":{"type":"string"},
								"context":{"type":"object","additionalProperties":{}}
							},
							"required":["status","id","context"]
						}}
					},
					"required":["type","title","status"],
					"additionalProperties":{}
				}},
				"responses":{"Problem":{
					"description":"Error",
					"content":{"application/problem+json":{"schema":{"$ref":"#/components/schemas/Problem"}}}
				}}
			}
		}`)
		testutil.Equal(t, "Version", openapi.Version, "3.1.0")
		testutil.Equal(t, "ProblemName", openapi.ProblemName, "Problem")
	})

	t.Run("should document the paths of the patterns", func(t *testing.T) {
		d := document(
			flam.RestEndpoint{Pattern: "GET /files/{path...}"},
			flam.RestEndpoint{Pattern: "POST /files/{path}"},
			flam.RestEndpoint{Pattern: "get\t/exact/{$}"},
			flam.RestEndpoint{Pattern: "DELETE example.com/hosted/{id}"},
			flam.RestEndpoint{Pattern: "GET /files/{path}", Summary: "hidden by the first"},
			flam.RestEndpoint{Pattern: "/no/method"},
			flam.RestEndpoint{Pattern: "PURGE /unknown"},
			flam.RestEndpoint{Pattern: "GET example.com"},
			flam.RestEndpoint{Pattern: "GET /a b"},
			flam.RestEndpoint{Pattern: ""},
		)
		testutil.Equal(t, "paths", len(d.Paths), 3)
		testutil.Equal(t, "files", len(d.Paths["/files/{path}"]), 2)
		testutil.Equal(t, "first kept", d.Paths["/files/{path}"]["get"].Summary, "")
		testutil.Equal(t, "post", d.Paths["/files/{path}"]["post"] != nil, true)
		testutil.Equal(t, "exact", d.Paths["/exact/"]["get"] != nil, true)
		equalJSON(t, "hosted", d.Paths["/hosted/{id}"]["delete"], `{
			"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
			"responses":{
				"200":{"description":"OK"},
				"default":{"$ref":"#/components/responses/Problem"}
			}
		}`)
	})

	t.Run("should document the operations of the endpoints", func(t *testing.T) {
		type body struct {
			Name string `json:"name"`
		}
		d := document(
			flam.RestEndpoint{ServiceId: 3, Id: 7, Pattern: "POST /users", Status: http.StatusCreated,
				Summary: "Create", Description: "Creates a user", Tags: []string{"users"}, Deprecated: true,
				Request: body{}, Response: &body{}},
			flam.RestEndpoint{Id: 8, Pattern: "DELETE /users/{id}", Status: http.StatusNoContent, Response: body{}},
			flam.RestEndpoint{Pattern: "PUT /users/{id}", Status: http.StatusNotModified, Response: body{}},
			flam.RestEndpoint{Pattern: "GET /events", Streaming: true, Response: body{}},
			flam.RestEndpoint{ServiceId: 3, Pattern: "PATCH /odd", Status: 299},
		)
		equalJSON(t, "create", d.Paths["/users"]["post"], `{
			"operationId":"s3.e7",
			"summary":"Create",
			"description":"Creates a user",
			"tags":["users"],
			"deprecated":true,
			"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/body"}}}},
			"responses":{
				"201":{"description":"Created","content":{"application/json":{"schema":{
					"type":"object",
					"properties":{"data":{"anyOf":[{"$ref":"#/components/schemas/body"},{"type":"null"}]}},
					"required":["data"]
				}}}},
				"default":{"$ref":"#/components/responses/Problem"}
			}
		}`)
		equalJSON(t, "delete", d.Paths["/users/{id}"]["delete"].Responses["204"], `{"description":"No Content"}`)
		testutil.Equal(t, "delete id", d.Paths["/users/{id}"]["delete"].OperationId, "e8")
		equalJSON(t, "put", d.Paths["/users/{id}"]["put"].Responses["304"], `{"description":"Not Modified"}`)
		testutil.Equal(t, "put id", d.Paths["/users/{id}"]["put"].OperationId, "")
		equalJSON(t, "events", d.Paths["/events"]["get"].Responses["200"],
			`{"description":"OK","content":{"text/event-stream":{"schema":{"type":"string"}}}}`)
		equalJSON(t, "odd", d.Paths["/odd"]["patch"].Responses["299"], `{"description":"Success"}`)
		testutil.Equal(t, "odd id", d.Paths["/odd"]["patch"].OperationId, "")
	})

	t.Run("should document the parameters of the path and of the query", func(t *testing.T) {
		type Range struct {
			From int `schema:"from"`
			To   *int
		}
		type Common struct {
			Lang string `schema:"lang"`
		}
		type params struct {
			Common
			Id      int        `schema:"id"`
			Search  string     `schema:"search,required"`
			Since   *time.Time `schema:"since"`
			Range   Range      `schema:"range"`
			Skip    string     `schema:"-"`
			Untaged bool
			Ids     []int `schema:"ids"`
		}
		want := `[
			{"name":"id","in":"path","required":true,"schema":{"type":"integer","format":"int64"}},
			{"name":"rest","in":"path","required":true,"schema":{"type":"string"}},
			{"name":"lang","in":"query","schema":{"type":"string"}},
			{"name":"search","in":"query","required":true,"schema":{"type":"string"}},
			{"name":"since","in":"query","schema":{"type":"string","format":"date-time"}},
			{"name":"range.from","in":"query","schema":{"type":"integer","format":"int64"}},
			{"name":"range.To","in":"query","schema":{"type":"integer","format":"int64"}},
			{"name":"Untaged","in":"query","schema":{"type":"boolean"}},
			{"name":"ids","in":"query","schema":{"type":"array","items":{"type":"integer","format":"int64"}}}
		]`
		for name, value := range map[string]any{"value": params{}, "pointer": &params{}} {
			d := document(flam.RestEndpoint{Pattern: "GET /users/{id}/{rest...}", Params: value})
			equalJSON(t, name, d.Paths["/users/{id}/{rest}"]["get"].Parameters, want)
		}

		d := document(flam.RestEndpoint{Pattern: "GET /users/{id}", Params: 1})
		equalJSON(t, "not a struct", d.Paths["/users/{id}"]["get"].Parameters,
			`[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}]`)
		d = document(flam.RestEndpoint{Pattern: "GET /users", Params: struct{}{}})
		testutil.Equal(t, "no parameter", d.Paths["/users"]["get"].Parameters, []*openapi.Parameter(nil))
	})

	t.Run("should write a single type as a string", func(t *testing.T) {
		data, err := json.Marshal(openapi.Types{"string"})
		testutil.NoError(t, err)
		testutil.Equal(t, "one", string(data), `"string"`)
		data, err = json.Marshal(openapi.Types{"string", "null"})
		testutil.NoError(t, err)
		testutil.Equal(t, "two", string(data), `["string","null"]`)
	})
}

// registers are the endpoint registers of the container.
type registers struct {
	dig.In

	Registers []flam.IRestEndpointRegister `group:"flam.rest.endpoints"`
}

// newRegister returns the register of an app with the config and the
// openapi bundles, and the configuration holding the values.
func newRegister(t *testing.T, values map[string]any) flam.IRestEndpointRegister {
	t.Helper()
	app := testutil.NewApp()
	for _, b := range []flam.IBundle{config.NewBundle(), openapi.NewBundle()} {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	bag := container.Bag{}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var register flam.IRestEndpointRegister
	testutil.NoError(t, app.Invoke(func(r registers) {
		testutil.Equal(t, "registers", len(r.Registers), 1)
		register = r.Registers[0]
	}))
	return register
}

// get serves a GET request of the path with the handler.
func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// router is a flam.IRestRouter not listing its endpoints.
type router struct{}

func (router) Handle(flam.RestEndpoint, http.Handler) error { return nil }

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"GroupConfigPath":       {openapi.GroupConfigPath, "flam.rest.openapi.group"},
		"PathConfigPath":        {openapi.PathConfigPath, "flam.rest.openapi.path"},
		"IdConfigPath":          {openapi.IdConfigPath, "flam.rest.openapi.id"},
		"TitleConfigPath":       {openapi.TitleConfigPath, "flam.rest.openapi.title"},
		"VersionConfigPath":     {openapi.VersionConfigPath, "flam.rest.openapi.version"},
		"DescriptionConfigPath": {openapi.DescriptionConfigPath, "flam.rest.openapi.description"},
		"ServersConfigPath":     {openapi.ServersConfigPath, "flam.rest.openapi.servers"},
		"UIConfigPath":          {openapi.UIConfigPath, "flam.rest.openapi.ui.enabled"},
		"UIPathConfigPath":      {openapi.UIPathConfigPath, "flam.rest.openapi.ui.path"},
		"UIIdConfigPath":        {openapi.UIIdConfigPath, "flam.rest.openapi.ui.id"},
		"UICDNConfigPath":       {openapi.UICDNConfigPath, "flam.rest.openapi.ui.cdn"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	if openapi.ErrRouter == nil {
		t.Error("ErrRouter = nil")
	}
}

func Test_Bundle(t *testing.T) {
	b := openapi.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.rest.openapi")
	app := testutil.NewApp()
	testutil.NoError(t, app.RegisterBundle(b))
	testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.openapi", "provider/1"})
}

func Test_Register(t *testing.T) {
	t.Run("should serve the document of the endpoints of the router, whatever their order", func(t *testing.T) {
		register := newRegister(t, nil)
		testutil.Equal(t, "Group()", register.Group(), "api")

		r, err := rest.NewRouter(4)
		testutil.NoError(t, err)
		testutil.NoError(t, register.Register(r))
		testutil.NoError(t, r.Handle(flam.RestEndpoint{Id: 1, Pattern: "GET /users", Response: []string{}}, http.NotFoundHandler()))

		rec := get(r, "/openapi.json")
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), "application/json")
		var doc map[string]any
		testutil.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc))
		equalJSON(t, "info", doc["info"], `{"title":"API","version":"1.0.0"}`)
		testutil.Equal(t, "servers", doc["servers"], nil)
		equalJSON(t, "paths", doc["paths"], `{
			"/openapi.json":{"get":{"summary":"OpenAPI document","responses":{
				"200":{"description":"OK"},
				"default":{"$ref":"#/components/responses/Problem"}
			}}},
			"/users":{"get":{"operationId":"s4.e1","responses":{
				"200":{"description":"OK","content":{"application/json":{"schema":{
					"type":"object","properties":{"data":{"type":"array","items":{"type":"string"}}},"required":["data"]
				}}}},
				"default":{"$ref":"#/components/responses/Problem"}
			}}}
		}`)

		testutil.NoError(t, r.Handle(flam.RestEndpoint{Pattern: "GET /later"}, http.NotFoundHandler()))
		testutil.Equal(t, "built once", get(r, "/openapi.json").Body.String(), rec.Body.String())
		testutil.Equal(t, "no ui", get(r, "/docs").Code, http.StatusNotFound)
	})

	t.Run("should take the configuration", func(t *testing.T) {
		register := newRegister(t, map[string]any{
			openapi.GroupConfigPath:       "public",
			openapi.PathConfigPath:        "/spec",
			openapi.IdConfigPath:          9,
			openapi.TitleConfigPath:       "Users </script>",
			openapi.VersionConfigPath:     "2.0.0",
			openapi.DescriptionConfigPath: "The users",
			openapi.ServersConfigPath:     []any{"https://a.example.com", "https://b.example.com"},
			openapi.UIConfigPath:          true,
			openapi.UIPathConfigPath:      "/ui",
			openapi.UIIdConfigPath:        10,
			openapi.UICDNConfigPath:       "https://cdn.example.com/swagger",
		})
		testutil.Equal(t, "Group()", register.Group(), "public")

		r, err := rest.NewRouter(0)
		testutil.NoError(t, err)
		testutil.NoError(t, register.Register(r))
		var doc map[string]any
		testutil.NoError(t, json.Unmarshal(get(r, "/spec").Body.Bytes(), &doc))
		equalJSON(t, "info", doc["info"], `{"title":"Users </script>","version":"2.0.0","description":"The users"}`)
		equalJSON(t, "servers", doc["servers"], `[{"url":"https://a.example.com"},{"url":"https://b.example.com"}]`)
		equalJSON(t, "operation ids", []any{
			doc["paths"].(map[string]any)["/spec"].(map[string]any)["get"].(map[string]any)["operationId"],
			doc["paths"].(map[string]any)["/ui"].(map[string]any)["get"].(map[string]any)["operationId"],
		}, `["e9","e10"]`)

		rec := get(r, "/ui")
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), "text/html; charset=utf-8")
		testutil.Equal(t, "page", rec.Body.String(), `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Users &lt;/script&gt;</title>
<link rel="stylesheet" href="https://cdn.example.com/swagger/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://cdn.example.com/swagger/swagger-ui-bundle.js" crossorigin></script>
<script>
window.onload = function () {
  window.ui = SwaggerUIBundle({url: "/spec", dom_id: "#swagger-ui"});
};
</script>
</body>
</html>
`)
	})

	t.Run("should load the default CDN", func(t *testing.T) {
		register := newRegister(t, map[string]any{openapi.UIConfigPath: true})
		r, err := rest.NewRouter(0)
		testutil.NoError(t, err)
		testutil.NoError(t, register.Register(r))
		body := get(r, "/docs").Body.String()
		testutil.Equal(t, "cdn", strings.Contains(body, `src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"`), true)
		testutil.Equal(t, "url", strings.Contains(body, `url: "/openapi.json"`), true)
	})

	t.Run("should fail with a router not listing its endpoints", func(t *testing.T) {
		testutil.Error(t, newRegister(t, nil).Register(router{}), openapi.ErrRouter,
			"unable to register openapi endpoints: rest router does not list its endpoints", map[string]any{})
	})

	t.Run("should fail with the error of the router", func(t *testing.T) {
		for name, values := range map[string]map[string]any{
			"document": {openapi.PathConfigPath: "no-slash"},
			"ui":       {openapi.UIConfigPath: true, openapi.UIPathConfigPath: "/openapi.json"},
		} {
			r, err := rest.NewRouter(0)
			testutil.NoError(t, err)
			err = newRegister(t, values).Register(r)
			if err == nil {
				t.Fatalf("%s: Register() did not fail", name)
			}
			testutil.Error(t, err, rest.ErrInvalidEndpoint, err.Error(), map[string]any{
				rest.ErrorFieldEndpoint: 0,
				rest.ErrorFieldPattern:  "GET " + map[string]string{"document": "no-slash", "ui": "/openapi.json"}[name],
				rest.ErrorFieldField:    "pattern",
			})
		}
	})
}
