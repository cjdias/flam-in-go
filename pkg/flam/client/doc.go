// Package client is the outgoing HTTP client feature: the bundle returned
// from [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IClientFactory] of an
// application, creating the clients described by the configuration at
// [ClientConfigPath] with the creators provided in the
// [ClientCreatorGroup] container group, and the
// [github.com/cjdias/flam-in-go/pkg/flam.IClientCircuitBreakerFactory]
// creating the circuit breakers described at [BreakerConfigPath] with the
// creators of the [BreakerCreatorGroup] group, such as:
//
//	flam:
//	  client:
//	    breakers:
//	      users: {driver: flam.client.breakers.driver.standard, failure_threshold: 3}
//	    clients:
//	      users:
//	        driver: flam.client.clients.driver.standard
//	        base_url: https://users.internal/v1/
//	        breaker: users
//	        retries: 2
//
// A client then sends its requests, under the protection of its circuit
// breaker, which fails them fast while the server keeps failing:
//
//	func (s *service) user(ctx context.Context, id string) (*http.Response, error) {
//		client, err := s.clients.Get("users")
//		if err != nil {
//			return nil, err
//		}
//		return client.Request(ctx, http.MethodGet, "users/"+id, nil)
//	}
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/client/clients] and
// [github.com/cjdias/flam-in-go/pkg/flam/client/breakers]. They import
// this package for its paths and groups, so this package never imports
// them.
package client
