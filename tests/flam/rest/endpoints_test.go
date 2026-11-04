package rest

import (
	"net/http"
	"sync"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Router_Endpoints(t *testing.T) {
	t.Run("should list nothing before a registration", func(t *testing.T) {
		router, err := rest.NewRouter(7)
		testutil.NoError(t, err)
		testutil.Equal(t, "Endpoints()", router.Endpoints(), []flam.RestEndpoint(nil))
	})

	t.Run("should list the endpoints registered, in order, with the service id", func(t *testing.T) {
		router, err := rest.NewRouter(7, &middleware{id: "outer", nilHandler: true, nilPattern: "GET /nil"})
		testutil.NoError(t, err)
		ok := http.HandlerFunc(handler)
		users := flam.RestEndpoint{ServiceId: 1, Id: 2, Pattern: "GET /users/{id}", Summary: "Get a user", Tags: []string{"users"}}
		testutil.NoError(t, router.Handle(users, ok))
		testutil.Equal(t, "no pattern", router.Handle(flam.RestEndpoint{Id: 3}, ok) != nil, true)
		testutil.Equal(t, "nil handler", router.Handle(flam.RestEndpoint{Id: 4, Pattern: "GET /nil"}, ok) != nil, true)
		testutil.Equal(t, "conflict", router.Handle(flam.RestEndpoint{Id: 5, Pattern: "GET /users/{id}"}, ok) != nil, true)
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 6, Pattern: "POST /users", Streaming: true}, ok))

		users.ServiceId = 7
		endpoints := router.Endpoints()
		testutil.Equal(t, "Endpoints()", endpoints, []flam.RestEndpoint{
			users,
			{ServiceId: 7, Id: 6, Pattern: "POST /users", Streaming: true},
		})

		endpoints[0].Id = 99
		testutil.Equal(t, "copy", router.Endpoints()[0].Id, 2)
		var lister flam.IRestEndpointLister = router
		testutil.Equal(t, "lister", len(lister.Endpoints()), 2)
	})

	t.Run("should list the endpoints while others are registered", func(t *testing.T) {
		router, err := rest.NewRouter(0)
		testutil.NoError(t, err)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				_ = router.Handle(flam.RestEndpoint{Id: i, Pattern: "GET /items/" + string(rune('a'+i))}, http.HandlerFunc(handler))
				_ = router.Endpoints()
			})
		}
		wg.Wait()
		testutil.Equal(t, "count", len(router.Endpoints()), 20)
	})
}
