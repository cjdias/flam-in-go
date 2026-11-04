// Package standard is the mongo connection driver of a MongoDB
// deployment, reached with the connection string of its URI field and the
// client options of its other fields, such as the credentials, the
// timeouts, the pool sizes and the TLS certificates.
//
// Creating a connection validates the options without waiting for the
// deployment, which the client reaches in the background: an unreachable
// one fails the operations instead, unless the ping field asks the
// creation to ping it, within the server selection timeout.
package standard
