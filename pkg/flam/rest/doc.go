// Package rest is the REST feature: the bundle returned from [NewBundle]
// provides the [github.com/cjdias/flam-in-go/pkg/flam.IRestServerFactory]
// of an application, along with the factories of the logger adaptors and
// of the middlewares of its servers, and adds its servers to the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog], which runs them, and
// shuts them down on an interrupt or a SIGTERM signal.
//
// Each server serves the endpoints of its group, registered on its
// [Router] by the
// [github.com/cjdias/flam-in-go/pkg/flam.IRestEndpointRegister] values
// the application provides in the [EndpointGroup] container group, such
// as:
//
//	func (r *users) Group() string { return "api" }
//
//	func (r *users) Register(router flam.IRestRouter) error {
//		return router.Handle(
//			flam.RestEndpoint{Id: 1, Pattern: "GET /users/{id}"},
//			http.HandlerFunc(r.get))
//	}
//
//	func (r *users) get(w http.ResponseWriter, req *http.Request) {
//		var params struct{ Id int `schema:"id"` }
//		if err := rest.DecodePath(r.decoder, req, &params); err != nil {
//			_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().
//				WithNewError(http.StatusBadRequest, err))
//			return
//		}
//		...
//		_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope(user))
//	}
//
// configured, with the drivers of the sub packages registered, as:
//
//	flam:
//	  rest:
//	    service_id: 7
//	    servers:
//	      api: {driver: flam.rest.servers.driver.standard, address: ":8080"}
//
// The handlers decode the requests with [DecodePath], [DecodeQuery] and
// [DecodeJSON], and answer with the
// [github.com/cjdias/flam-in-go/pkg/flam/envelope.Envelope] values written
// by [WriteEnvelope], whose errors are identified by the ids of the
// service and of the endpoint.
//
// [Validate] and [DecodeJSONAndValidate] check the decoded values with a
// [github.com/cjdias/flam-in-go/pkg/flam.IValidator], returning the
// envelope answering an invalid one, best of a validator with the
// converter of
// [github.com/cjdias/flam-in-go/pkg/flam/validator/converters/envelope],
// which identifies each error by its param and rule. [DecodePage] decodes
// the [Page] of a collection asked for by the query, bounded by
// [DefaultPageCount] and [MaxPageCount], and [Page.Envelope] answers its
// items with their
// [github.com/cjdias/flam-in-go/pkg/flam/envelope.Pagination].
//
// A [Router] lists the endpoints it registered, as a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestEndpointLister], so that
// [github.com/cjdias/flam-in-go/pkg/flam/rest/openapi] documents them
// from the documentation fields of their
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint].
//
// An endpoint streaming its response sets the Streaming field of its
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint], so that the
// timeout middleware passes its requests through rather than buffering
// them, and is not to be wrapped in the middlewares compressing the
// responses, or computing their ETag. [NewEventStream] streams
// Server-Sent Events. A WebSocket takes the connection over with the
// Hijack method of an [http.ResponseController] made of the writer given
// to the handler, which reaches the writer of the server through the
// Unwrap methods of the writers of the middlewares, such as the one of
// [NewResponseWriter], the upgrade being done by a WebSocket module of the
// application.
//
// As a process of the kennel, a server also reports its start and its end
// to its watchdog logger adaptor, configured at
// [watchdog.ProcessConfigPath] by its id, along with its own logger
// adaptor, which reports its address, its errors and its shutdown too. To
// log them once, give the server no watchdog logger adaptor:
//
//	flam:
//	  watchdog:
//	    processes:
//	      api: {logger: ""}
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/servers],
// [github.com/cjdias/flam-in-go/pkg/flam/rest/loggers] and
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares]. They import
// this package for its paths and groups, so this package never imports
// them.
package rest
