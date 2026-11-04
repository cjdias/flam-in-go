// Package standard is the REST server driver serving HTTP, or HTTPS when
// given a certificate, with the [net/http] server: each server listens on
// its address when run by the kennel, serving the endpoints of its group
// through its middlewares, and reporting its lifecycle to its logger
// adaptor, until the kennel closes it, letting the requests under way end
// within its shutdown timeout.
package standard
