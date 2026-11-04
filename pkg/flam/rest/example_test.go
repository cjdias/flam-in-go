package rest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/schema"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// users is an endpoint register serving the users by their id.
type users struct {
	decoder *schema.Decoder
}

func (r *users) Group() string { return "api" }

func (r *users) Register(router flam.IRestRouter) error {
	return router.Handle(
		flam.RestEndpoint{Id: 2, Pattern: "GET /users/{id}"},
		http.HandlerFunc(r.get))
}

func (r *users) get(w http.ResponseWriter, req *http.Request) {
	var params struct {
		Id int `schema:"id"`
	}
	if err := rest.DecodePath(r.decoder, req, &params); err != nil {
		_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().
			WithNewError(http.StatusBadRequest, err))
		return
	}
	_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope(map[string]any{"id": params.Id}))
}

func ExampleNewRouter() {
	router, err := rest.NewRouter(1)
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := (&users{decoder: rest.NewDecoder()}).Register(router); err != nil {
		fmt.Println(err)
		return
	}

	for _, target := range []string{"/users/42", "/orders"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		fmt.Println(w.Code, w.Header().Get("Content-Type"))
		fmt.Println(w.Body.String())
	}
	// Output:
	// 200 application/json
	// {"data":{"id":42}}
	// 404 application/problem+json
	// {"detail":"rest route not found","id":"s:1","status":404,"title":"Not Found","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404"}
}
