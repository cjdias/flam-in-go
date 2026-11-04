package rest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Page_Declarations(t *testing.T) {
	testutil.Equal(t, "DefaultPageCount", rest.DefaultPageCount, 20)
	testutil.Equal(t, "MaxPageCount", rest.MaxPageCount, 100)
}

func Test_DecodePage(t *testing.T) {
	for name, tc := range map[string]struct {
		query string
		want  rest.Page
	}{
		"no query":          {"", rest.Page{Count: rest.DefaultPageCount}},
		"a page":            {"?search=jo&start=40&count=10", rest.Page{Search: "jo", Start: 40, Count: 10}},
		"a negative start":  {"?start=-5&count=5", rest.Page{Start: 0, Count: 5}},
		"a count of zero":   {"?count=0", rest.Page{Count: rest.DefaultPageCount}},
		"a negative count":  {"?count=-3", rest.Page{Count: rest.DefaultPageCount}},
		"too large a count": {"?count=1000", rest.Page{Count: rest.MaxPageCount}},
		"the largest count": {"?count=100", rest.Page{Count: rest.MaxPageCount}},
		"an encoded search": {"?search=50%25+off&other=1", rest.Page{Search: "50% off", Count: rest.DefaultPageCount}},
	} {
		t.Run("should decode "+name, func(t *testing.T) {
			page, err := rest.DecodePage(rest.NewDecoder(), httptest.NewRequest(http.MethodGet, "/users"+tc.query, nil))
			testutil.NoError(t, err)
			testutil.Equal(t, "page", page, tc.want)
		})
	}

	t.Run("should fail on an invalid query", func(t *testing.T) {
		page, err := rest.DecodePage(rest.NewDecoder(), httptest.NewRequest(http.MethodGet, "/users?start=x", nil))
		testutil.Equal(t, "page", page, rest.Page{})
		var e *flam.Error
		if !errors.Is(err, rest.ErrDecode) || !errors.As(err, &e) {
			t.Fatalf("DecodePage() = %v, want it to wrap ErrDecode", err)
		}
		testutil.Context(t, e, map[string]any{rest.ErrorFieldSource: rest.SourceQuery})
	})
}

func Test_Page_Bound(t *testing.T) {
	for name, tc := range map[string]struct {
		page                   rest.Page
		defaultCount, maxCount int
		want                   rest.Page
	}{
		"a page within the bounds": {rest.Page{Search: "a", Start: 3, Count: 5}, 10, 50, rest.Page{Search: "a", Start: 3, Count: 5}},
		"a negative start":         {rest.Page{Start: -1, Count: 5}, 10, 50, rest.Page{Count: 5}},
		"no count":                 {rest.Page{}, 10, 50, rest.Page{Count: 10}},
		"too large a count":        {rest.Page{Count: 51}, 10, 50, rest.Page{Count: 50}},
		"too large a default":      {rest.Page{}, 80, 50, rest.Page{Count: 50}},
		"no max":                   {rest.Page{Count: 5000}, 10, 0, rest.Page{Count: 5000}},
	} {
		t.Run("should bound "+name, func(t *testing.T) {
			testutil.Equal(t, "Bound()", tc.page.Bound(tc.defaultCount, tc.maxCount), tc.want)
		})
	}

	t.Run("should leave the page unchanged", func(t *testing.T) {
		page := rest.Page{Start: -1}
		page.Bound(10, 50)
		testutil.Equal(t, "page", page, rest.Page{Start: -1})
	})
}

func Test_Page_Envelope(t *testing.T) {
	page := rest.Page{Search: "a b", Start: 10, Count: 10}

	t.Run("should link the pages to the path of the request", func(t *testing.T) {
		env := page.Envelope(httptest.NewRequest(http.MethodGet, "/my%20users?search=a+b&start=10&count=10", nil), []int{1}, 35)
		testutil.Equal(t, "env", env, envelope.NewEnvelope([]int{1}).WithPagination("a b", 10, 10, 35, "/my%20users"))
		pagination, ok := env.Filter.(*envelope.Pagination)
		if !ok {
			t.Fatalf("Filter = %#v, want a *envelope.Pagination", env.Filter)
		}
		testutil.Equal(t, "Prev", pagination.Prev, "/my%20users?count=10&search=a+b&start=0")
		testutil.Equal(t, "Next", pagination.Next, "/my%20users?count=10&search=a+b&start=20")
	})

	t.Run("should leave the links relative without a request", func(t *testing.T) {
		testutil.Equal(t, "env", page.Envelope(nil, "data", 35), envelope.NewEnvelope("data").WithPagination("a b", 10, 10, 35))
		testutil.Equal(t, "env", page.Envelope(&http.Request{}, "data", 35), envelope.NewEnvelope("data").WithPagination("a b", 10, 10, 35))
	})

	t.Run("should be written as it is", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()
		testutil.NoError(t, rest.WriteEnvelope(w, req, rest.Page{Count: 2}.Envelope(req, []string{"a", "b"}, 3)))
		testutil.Equal(t, "status", w.Code, http.StatusOK)
		testutil.Equal(t, "body", w.Body.String(),
			`{"data":["a","b"],"filter":{"start":0,"count":2,"total":3,"next":"/users?count=2\u0026start=2"}}`)
	})
}
