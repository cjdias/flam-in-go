package envelope

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// marshal returns the JSON of the value.
func marshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	testutil.NoError(t, err)
	return string(data)
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ContextFieldMessage", envelope.ContextFieldMessage, "message")
}

func Test_NewError(t *testing.T) {
	t.Run("plain error", func(t *testing.T) {
		e := envelope.NewError(http.StatusBadRequest, errBoom)
		testutil.Equal(t, "Status", e.Status, http.StatusBadRequest)
		testutil.Equal(t, "Id", e.Id, "")
		testutil.Equal(t, "Context", e.Context, container.Bag{"message": "boom"})
	})

	t.Run("nil error", func(t *testing.T) {
		e := envelope.NewError(http.StatusNotFound, nil)
		testutil.Equal(t, "Context", e.Context, container.Bag{})
		testutil.Equal(t, "Error", e.Error(), "Not Found")
	})

	t.Run("nil flam error", func(t *testing.T) {
		var nilErr *flam.Error
		e := envelope.NewError(http.StatusNotFound, nilErr)
		testutil.Equal(t, "Context", e.Context, container.Bag{})
	})

	t.Run("flam error in the chain", func(t *testing.T) {
		inner := flam.NewWrappedError(errBoom, "unable", map[string]any{"id": 1, "path": "a"})
		e := envelope.NewError(http.StatusConflict, fmt.Errorf("outer: %w", inner))
		testutil.Equal(t, "Context", e.Context, container.Bag{
			"message": "outer: unable: boom",
			"id":      1,
			"path":    "a",
		})
	})

	t.Run("flam error context is copied", func(t *testing.T) {
		inner := flam.NewError("unable", map[string]any{"list": []any{1}})
		e := envelope.NewError(http.StatusConflict, inner)
		e.Context["list"].([]any)[0] = 2
		testutil.Equal(t, "error context", inner.Context()["list"], []any{1})
	})

	t.Run("maps merged in order", func(t *testing.T) {
		inner := flam.NewError("unable", map[string]any{"id": 1, "path": "a"})
		e := envelope.NewError(http.StatusConflict, inner,
			map[string]any{"id": 2, "extra": true},
			nil,
			container.Bag{"id": 3, "message": "replaced"})
		testutil.Equal(t, "Context", e.Context, container.Bag{
			"message": "replaced",
			"id":      3,
			"path":    "a",
			"extra":   true,
		})
	})

	t.Run("maps are copied", func(t *testing.T) {
		ctx := map[string]any{"id": 1}
		e := envelope.NewError(http.StatusConflict, errBoom, ctx)
		ctx["id"] = 2
		testutil.Equal(t, "id", e.Context["id"], 1)
	})
}

func Test_Error_Error(t *testing.T) {
	for name, tc := range map[string]struct {
		err  envelope.Error
		want string
	}{
		"string message": {envelope.NewError(http.StatusBadRequest, errBoom), "boom"},
		"other message":  {envelope.Error{Context: container.Bag{"message": 42}}, "42"},
		"no message":     {envelope.Error{Status: http.StatusTeapot}, "I'm a teapot"},
		"nil message":    {envelope.Error{Status: http.StatusGone, Context: container.Bag{"message": nil}}, "Gone"},
		"unknown status": {envelope.Error{Status: 999}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.Equal(t, "Error", tc.err.Error(), tc.want)
		})
	}

	t.Run("is an error", func(t *testing.T) {
		var err error = envelope.NewError(http.StatusBadRequest, errBoom)
		var target envelope.Error
		testutil.Equal(t, "As", errors.As(err, &target), true)
		testutil.Equal(t, "Status", target.Status, http.StatusBadRequest)
	})
}

func Test_Error_WithStatus(t *testing.T) {
	base := envelope.NewError(http.StatusBadRequest, errBoom)
	e := base.WithStatus(http.StatusConflict)
	testutil.Equal(t, "Status", e.Status, http.StatusConflict)
	testutil.Equal(t, "base Status", base.Status, http.StatusBadRequest)
}

func Test_Error_Id(t *testing.T) {
	base := envelope.NewError(http.StatusBadRequest, errBoom)
	for name, tc := range map[string]struct {
		err  envelope.Error
		want string
	}{
		"none":            {base, ""},
		"service":         {base.WithServiceId(1), "s:1"},
		"endpoint":        {base.WithEndpointId(2), "e:2"},
		"param":           {base.WithParamId(3), "p:3"},
		"numeric code":    {base.WithErrorId(4), "c:4"},
		"numeric text":    {base.WithErrorId("05"), "c:5"},
		"zero code":       {base.WithErrorId(0), "c:0"},
		"text code":       {base.WithErrorId("required"), "required"},
		"empty code":      {base.WithErrorId(""), ""},
		"negative ids":    {base.WithServiceId(-1).WithEndpointId(-2), "s:-1.e:-2"},
		"service, code":   {base.WithServiceId(1).WithErrorId(4), "s:1.c:4"},
		"endpoint, param": {base.WithEndpointId(2).WithParamId(3), "e:2.p:3"},
		"all":             {base.WithErrorId("x").WithParamId(3).WithEndpointId(2).WithServiceId(1), "s:1.e:2.p:3.x"},
		"reset to zero":   {base.WithServiceId(1).WithEndpointId(2).WithServiceId(0), "e:2"},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.Equal(t, "Id", tc.err.Id, tc.want)
		})
	}

	t.Run("copies", func(t *testing.T) {
		e := base.WithServiceId(1)
		_ = e.WithEndpointId(2)
		testutil.Equal(t, "base Id", base.Id, "")
		testutil.Equal(t, "Id", e.Id, "s:1")
	})
}

func Test_Error_WithContext(t *testing.T) {
	t.Run("set on a copy", func(t *testing.T) {
		base := envelope.NewError(http.StatusBadRequest, errBoom)
		e := base.WithContext("field", "name").WithContext("message", "replaced")
		testutil.Equal(t, "Context", e.Context, container.Bag{"message": "replaced", "field": "name"})
		testutil.Equal(t, "base Context", base.Context, container.Bag{"message": "boom"})
	})

	t.Run("nil context", func(t *testing.T) {
		e := envelope.Error{}.WithContext("field", nil)
		testutil.Equal(t, "Context", e.Context, container.Bag{"field": nil})
	})
}

func Test_Error_JSON(t *testing.T) {
	e := envelope.NewError(http.StatusBadRequest, errBoom).WithServiceId(1).WithErrorId(2)
	testutil.Equal(t, "json", marshal(t, e), `{"status":400,"id":"s:1.c:2","context":{"message":"boom"}}`)
}

func Test_NewEnvelope(t *testing.T) {
	testutil.Equal(t, "no data", envelope.NewEnvelope(), envelope.Envelope{})
	testutil.Equal(t, "data", envelope.NewEnvelope("a"), envelope.Envelope{Data: "a"})
	testutil.Equal(t, "extra data", envelope.NewEnvelope("a", "b"), envelope.Envelope{Data: "a"})
}

func Test_Envelope_WithError(t *testing.T) {
	first := envelope.NewError(http.StatusBadRequest, errBoom)
	second := envelope.NewError(http.StatusConflict, errBoom)

	t.Run("appends on a copy", func(t *testing.T) {
		base := envelope.NewEnvelope("data")
		env := base.WithError(first).WithError(second)
		testutil.Equal(t, "Errors", env.Errors, []envelope.Error{first, second})
		testutil.Equal(t, "Data", env.Data, "data")
		testutil.Equal(t, "base Errors", len(base.Errors), 0)
	})

	t.Run("copies do not share their errors", func(t *testing.T) {
		base := envelope.NewEnvelope().WithError(first)
		base.Errors = append(make([]envelope.Error, 0, 4), base.Errors...)
		a := base.WithError(second)
		b := base.WithError(first)
		testutil.Equal(t, "a Errors", a.Errors, []envelope.Error{first, second})
		testutil.Equal(t, "b Errors", b.Errors, []envelope.Error{first, first})
	})

	t.Run("new error", func(t *testing.T) {
		env := envelope.NewEnvelope().WithNewError(http.StatusBadRequest, errBoom, map[string]any{"id": 1})
		testutil.Equal(t, "Errors", env.Errors, []envelope.Error{
			envelope.NewError(http.StatusBadRequest, errBoom, map[string]any{"id": 1}),
		})
	})
}

func Test_Envelope_Ids(t *testing.T) {
	base := envelope.NewEnvelope().
		WithNewError(http.StatusBadRequest, errBoom).
		WithError(envelope.NewError(http.StatusConflict, errBoom).WithParamId(3))

	env := base.WithServiceId(1).WithEndpointId(2)
	testutil.Equal(t, "first Id", env.Errors[0].Id, "s:1.e:2")
	testutil.Equal(t, "second Id", env.Errors[1].Id, "s:1.e:2.p:3")
	testutil.Equal(t, "base first Id", base.Errors[0].Id, "")
	testutil.Equal(t, "base second Id", base.Errors[1].Id, "p:3")

	testutil.Equal(t, "no errors", envelope.NewEnvelope().WithServiceId(1).WithEndpointId(2), envelope.Envelope{})
}

func Test_Envelope_WithPagination(t *testing.T) {
	for name, tc := range map[string]struct {
		search              string
		start, count, total int64
		path                []string
		want                envelope.Pagination
	}{
		"single page": {
			"", 0, 10, 5, nil,
			envelope.Pagination{Count: 10, Total: 5},
		},
		"first page": {
			"", 0, 10, 25, nil,
			envelope.Pagination{Count: 10, Total: 25, Next: "?count=10&start=10"},
		},
		"middle page": {
			"", 10, 10, 25, nil,
			envelope.Pagination{Start: 10, Count: 10, Total: 25, Prev: "?count=10&start=0", Next: "?count=10&start=20"},
		},
		"last page": {
			"", 20, 10, 25, nil,
			envelope.Pagination{Start: 20, Count: 10, Total: 25, Prev: "?count=10&start=10"},
		},
		"exact last page": {
			"", 10, 10, 20, nil,
			envelope.Pagination{Start: 10, Count: 10, Total: 20, Prev: "?count=10&start=0"},
		},
		"unaligned start": {
			"", 5, 10, 25, nil,
			envelope.Pagination{Start: 5, Count: 10, Total: 25, Prev: "?count=10&start=0", Next: "?count=10&start=15"},
		},
		"past the end": {
			"", 30, 10, 25, nil,
			envelope.Pagination{Start: 30, Count: 10, Total: 25, Prev: "?count=10&start=20"},
		},
		"zero count": {
			"", 10, 0, 25, nil,
			envelope.Pagination{Start: 10, Total: 25},
		},
		"negative count": {
			"", 10, -5, 25, nil,
			envelope.Pagination{Start: 10, Count: -5, Total: 25},
		},
		"search encoded": {
			"a&b=c d", 10, 10, 25, nil,
			envelope.Pagination{
				Search: "a&b=c d", Start: 10, Count: 10, Total: 25,
				Prev: "?count=10&search=a%26b%3Dc+d&start=0",
				Next: "?count=10&search=a%26b%3Dc+d&start=20",
			},
		},
		"path": {
			"", 10, 10, 25, []string{"/users"},
			envelope.Pagination{Start: 10, Count: 10, Total: 25, Prev: "/users?count=10&start=0", Next: "/users?count=10&start=20"},
		},
		"first path only": {
			"", 0, 10, 25, []string{"/users", "/other"},
			envelope.Pagination{Count: 10, Total: 25, Next: "/users?count=10&start=10"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := envelope.NewEnvelope("data").WithPagination(tc.search, tc.start, tc.count, tc.total, tc.path...)
			got, ok := env.Filter.(*envelope.Pagination)
			testutil.Equal(t, "is pagination", ok, true)
			testutil.Equal(t, "Pagination", *got, tc.want)
			testutil.Equal(t, "Data", env.Data, "data")
		})
	}

	t.Run("path slice left unchanged", func(t *testing.T) {
		backing := []string{"/users", "kept"}
		_ = envelope.NewEnvelope().WithPagination("", 0, 10, 25, backing[:1]...)
		testutil.Equal(t, "spare slot", backing[1], "kept")
	})

	t.Run("copy", func(t *testing.T) {
		base := envelope.NewEnvelope()
		_ = base.WithPagination("", 0, 10, 25)
		testutil.Equal(t, "base Filter", base.Filter, nil)
	})
}

func Test_Envelope_JSON(t *testing.T) {
	testutil.Equal(t, "empty", marshal(t, envelope.NewEnvelope()), `{}`)

	env := envelope.NewEnvelope([]int{1}).
		WithNewError(http.StatusBadRequest, errBoom).
		WithServiceId(1).
		WithPagination("", 0, 1, 2)
	testutil.Equal(t, "full", marshal(t, env),
		`{"errors":[{"status":400,"id":"s:1","context":{"message":"boom"}}],`+
			`"data":[1],`+
			`"filter":{"start":0,"count":1,"total":2,"next":"?count=1`+"\\u0026"+`start=1"}}`)
}

func Test_NewProblem(t *testing.T) {
	const base = "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/"
	testutil.Equal(t, "ProblemTypeBase", envelope.ProblemTypeBase, base)
	testutil.Equal(t, "ContentTypeProblem", envelope.ContentTypeProblem, "application/problem+json")

	t.Run("no error", func(t *testing.T) {
		testutil.Equal(t, "problem", envelope.NewProblem(envelope.NewEnvelope("x")), envelope.Problem{
			Type:       base + "500",
			Title:      "Internal Server Error",
			Status:     http.StatusInternalServerError,
			Extensions: container.Bag{},
		})
	})

	t.Run("first error", func(t *testing.T) {
		env := envelope.NewEnvelope().
			WithNewError(http.StatusConflict, errBoom, map[string]any{"field": "name"}).
			WithServiceId(1)
		testutil.Equal(t, "problem", envelope.NewProblem(env), envelope.Problem{
			Type:       base + "409",
			Title:      "Conflict",
			Status:     http.StatusConflict,
			Detail:     "boom",
			Extensions: container.Bag{"id": "s:1", "field": "name"},
		})
		testutil.Equal(t, "error context", env.Errors[0].Context, container.Bag{"message": "boom", "field": "name"})
	})

	t.Run("error without message", func(t *testing.T) {
		env := envelope.NewEnvelope().WithError(envelope.Error{Status: http.StatusTeapot})
		problem := envelope.NewProblem(env)
		testutil.Equal(t, "Detail", problem.Detail, "")
		testutil.Equal(t, "Extensions", problem.Extensions, container.Bag{})
		testutil.Equal(t, "json", marshal(t, problem),
			`{"status":418,"title":"I'm a teapot","type":"`+base+`418"}`)
	})

	t.Run("invalid status", func(t *testing.T) {
		for _, status := range []int{0, 100, 200, 302, 399, 600, 1000} {
			problem := envelope.NewProblem(envelope.NewEnvelope().WithNewError(status, errBoom))
			testutil.Equal(t, "Status", problem.Status, http.StatusInternalServerError)
			testutil.Equal(t, "Type", problem.Type, base+"500")
		}
	})

	t.Run("several errors", func(t *testing.T) {
		env := envelope.NewEnvelope().
			WithNewError(http.StatusBadRequest, errBoom).
			WithNewError(http.StatusConflict, errors.New("other"))
		problem := envelope.NewProblem(env)
		testutil.Equal(t, "Status", problem.Status, http.StatusBadRequest)
		testutil.Equal(t, "Extensions", problem.Extensions, container.Bag{"errors": env.Errors})
	})
}

func Test_Problem_JSON(t *testing.T) {
	testutil.Equal(t, "empty", marshal(t, envelope.Problem{}), `{}`)
	testutil.Equal(t, "full", marshal(t, envelope.Problem{
		Type:       "t",
		Title:      "x",
		Status:     400,
		Detail:     "d",
		Instance:   "/i",
		Extensions: container.Bag{"status": "replaced", "type": "replaced", "id": "s:1"},
	}), `{"detail":"d","id":"s:1","instance":"/i","status":400,"title":"x","type":"t"}`)
	testutil.Equal(t, "extensions kept", marshal(t, envelope.Problem{Extensions: container.Bag{"status": 1}}), `{"status":1}`)

	_, err := json.Marshal(envelope.Problem{Extensions: container.Bag{"bad": make(chan int)}})
	if err == nil {
		t.Error("Marshal() did not fail")
	}
}
