// Package envelope holds the [Envelope] wrapping the responses of a REST
// API: their data, the [Pagination] or other filter selecting it, and the
// [Error] values they report, identified by the ids of the service, the
// endpoint, the param and the error code. A response reporting errors is
// written as the RFC 9457 [Problem] of its first error, returned by
// [NewProblem].
//
// Envelopes and their errors are values: every With method returns a
// changed copy, leaving the receiver, and any other copy of it, unchanged,
// so that a base envelope can be shared and specialised.
//
//	env := envelope.NewEnvelope().
//		WithNewError(http.StatusNotFound, err).
//		WithServiceId(1).
//		WithEndpointId(2)
package envelope
