package rest

import (
	"encoding/json"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
)

// WriteEnvelope writes the envelope as the JSON response to the request.
//
// The ids of the service and of the endpoint held by the context of the
// request, read by [EndpointOf], are set on the errors of the envelope. An
// envelope with errors is written as the [envelope.Problem] of its first
// error, of the [envelope.ContentTypeProblem] content type, with its
// status, or 500 when out of the 400 to 599 range. An envelope with no
// error is written as it is, with the status of the endpoint, or 200 when
// unset. A response of a status allowing no body, such as 204, is written
// without body.
//
// It fails with ErrEncode when the envelope cannot be encoded, writing
// nothing, and with ErrWrite when the response cannot be written.
func WriteEnvelope(
	w http.ResponseWriter,
	req *http.Request,
	env envelope.Envelope,
) error {
	endpoint, _ := EndpointOf(req.Context())
	env = env.WithServiceId(endpoint.ServiceId).WithEndpointId(endpoint.Id)

	status, contentType, value := http.StatusOK, contentTypeJSON, any(env)
	switch {
	case len(env.Errors) > 0:
		problem := envelope.NewProblem(env)
		status, contentType, value = problem.Status, envelope.ContentTypeProblem, problem
	case endpoint.Status != 0:
		status = endpoint.Status
	}

	data, err := json.Marshal(value)
	if err != nil {
		return newErrJoined(ErrEncode, writeEnvelopeMessage, err)
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if !bodyAllowed(status) {
		return nil
	}
	if _, err := w.Write(data); err != nil {
		return newErrJoined(ErrWrite, writeEnvelopeMessage, err)
	}
	return nil
}

// writeEnvelopeMessage is the message of the errors returned by
// WriteEnvelope.
const writeEnvelopeMessage = "unable to write rest envelope"

// bodyAllowed reports whether a response of the status, a final one, may
// have a body.
func bodyAllowed(
	status int,
) bool {
	return status != http.StatusNoContent && status != http.StatusNotModified
}
