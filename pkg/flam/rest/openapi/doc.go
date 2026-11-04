// Package openapi documents the REST endpoints as an OpenAPI 3.1
// document: [NewDocument] returns the document of a list of
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint] values, from the
// patterns of the endpoints and their documentation fields, such as:
//
//	router.Handle(flam.RestEndpoint{
//		Id:       2,
//		Pattern:  "POST /users",
//		Status:   http.StatusCreated,
//		Summary:  "Create a user",
//		Tags:     []string{"users"},
//		Request:  UserRequest{},
//		Response: User{},
//	}, http.HandlerFunc(r.create))
//
// the JSON schemas of the bodies being generated from the Go types of the
// Params, Request and Response values, the successful responses holding
// the envelope of their data, and the errors the RFC 9457 problems
// written by [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope].
//
// The bundle returned from [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IRestEndpointRegister] serving,
// on the servers of its group, the one at [GroupConfigPath], "api" unless
// set, the JSON of the document of the endpoints of each server at the
// path at [PathConfigPath], "/openapi.json" unless set, and, when enabled
// at [UIConfigPath], the Swagger UI page of the document at the path at
// [UIPathConfigPath], "/docs" unless set, loading its files from the CDN
// at [UICDNConfigPath]:
//
//	flam:
//	  rest:
//	    openapi:
//	      title: Users
//	      version: 1.2.0
//	      servers: [https://api.example.com]
//	      ui: {enabled: true}
//
// The document is built at the first request, from the endpoints the
// router of the server lists as a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestEndpointLister], as the
// [github.com/cjdias/flam-in-go/pkg/flam/rest.Router] does, so that it
// documents the endpoints of every register, whatever their order, its
// own included.
package openapi
