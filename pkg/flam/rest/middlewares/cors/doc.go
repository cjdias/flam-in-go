// Package cors is the REST middleware driver implementing the Cross-Origin
// Resource Sharing protocol, telling the browsers which other origins may
// call the endpoints, with which methods and headers, and which headers of
// the responses they may read.
//
// A request without Origin header is served as it is. A request from an
// origin listed by [ConfigFieldAllowedOrigins], whatever its case, or from
// any origin when the list holds "*", is served with the
// Access-Control-Allow-Origin header, "*" when any origin is allowed and
// the origin of the request otherwise, along with
// Access-Control-Allow-Credentials when [ConfigFieldAllowCredentials]
// holds true, and Access-Control-Expose-Headers when
// [ConfigFieldExposedHeaders] lists headers. A request from another origin
// is served without them, so that the browser hides its response from the
// calling page.
//
// A preflight request, of the OPTIONS method with an
// Access-Control-Request-Method header, is answered by the middleware
// itself with a 204 response, without calling the handler: when the
// origin, the requested method, listed by [ConfigFieldAllowedMethods], and
// every requested header, listed by [ConfigFieldAllowedHeaders] whatever
// their case, or any header when the list holds "*", are allowed, the
// response holds the headers allowing the request, along with
// Access-Control-Allow-Methods, Access-Control-Allow-Headers, and
// Access-Control-Max-Age when [ConfigFieldMaxAge] is not 0, and holds none
// of them otherwise, so that the browser does not send the request. The
// preflight requests to the path of an endpoint whose pattern does not
// accept the OPTIONS method, or to no endpoint, are answered too, as the
// router wraps its handler of the requests matching no pattern in the
// middlewares.
//
// The responses vary with the Origin header, unless any origin is
// allowed, and the answers to the preflight requests with the Origin,
// Access-Control-Request-Method and Access-Control-Request-Headers
// headers: the Vary header of the responses lists them, so that the
// caches keep them apart.
//
// The credentials, cookies and authorization headers, can only be allowed
// for listed origins and exposed headers, as the browsers ignore the "*"
// wildcard along with them: the creation of a middleware allowing the
// credentials along with the "*" origin or exposed header fails with
// [ErrWildcardCredentials].
//
// List the middleware before the middlewares answering the requests
// themselves, such as an authentication middleware, so that their answers
// hold the CORS headers, and so that it answers the preflight requests,
// which carry no credentials, before them.
package cors
