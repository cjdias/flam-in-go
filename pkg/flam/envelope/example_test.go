package envelope_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
)

func ExampleNewProblem() {
	env := envelope.NewEnvelope().
		WithError(envelope.NewError(http.StatusNotFound, errors.New("user not found")).
			WithErrorId(3).
			WithContext("user", 42)).
		WithServiceId(1).
		WithEndpointId(2)

	fmt.Println(env.Errors[0].Id)

	out, err := json.Marshal(envelope.NewProblem(env))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(out))
	// Output:
	// s:1.e:2.c:3
	// {"detail":"user not found","id":"s:1.e:2.c:3","status":404,"title":"Not Found","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404","user":42}
}

func ExampleEnvelope_WithPagination() {
	env := envelope.NewEnvelope([]string{"c", "d"}).
		WithPagination("go", 2, 2, 5, "/users")

	page := env.Filter.(*envelope.Pagination)
	fmt.Println(page.Start, page.Count, page.Total)
	fmt.Println(page.Prev)
	fmt.Println(page.Next)
	// Output:
	// 2 2 5
	// /users?count=2&search=go&start=0
	// /users?count=2&search=go&start=4
}
