package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/schema"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// EndpointOf returns the endpoint held by the context of a request served
// by a [Router], and whether it holds one.
func EndpointOf(
	ctx context.Context,
) (flam.RestEndpoint, bool) {
	if ctx == nil {
		return flam.RestEndpoint{}, false
	}
	endpoint, ok := ctx.Value(endpointKey{}).(flam.RestEndpoint)
	return endpoint, ok
}

// NewDecoder returns the decoder of the path values and of the query of
// the requests, ignoring the keys matching no field of the decoded value,
// as provided to the container by the bundle returned from [NewBundle].
func NewDecoder() *schema.Decoder {
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	return decoder
}

// DecodePath decodes the path values of the request, named by the
// wildcards of the pattern it matched, such as "id" for "/users/{id}", into
// the value, a pointer to a struct, with the decoder. It fails with
// ErrDecode, with [SourcePath] in its context.
func DecodePath(
	decoder *schema.Decoder,
	req *http.Request,
	v any,
) error {
	values := url.Values{}
	for _, name := range wildcards(req.Pattern) {
		values.Set(name, req.PathValue(name))
	}
	return decode(decoder, v, values, SourcePath)
}

// DecodeQuery decodes the query of the URL of the request into the value,
// a pointer to a struct, with the decoder. It fails with ErrDecode, with
// [SourceQuery] in its context.
func DecodeQuery(
	decoder *schema.Decoder,
	req *http.Request,
	v any,
) error {
	return decode(decoder, v, req.URL.Query(), SourceQuery)
}

// DecodeJSON decodes the JSON body of the request into the value. It fails
// with ErrDecode, with [SourceBody] in its context, when the body, read
// whole, is not a single JSON value of the type of the value, such as when
// it is empty. A request body of no limit should be bounded beforehand,
// such as with [http.MaxBytesReader].
func DecodeJSON(
	req *http.Request,
	v any,
) error {
	body := req.Body
	if body == nil {
		body = http.NoBody
	}
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(v); err != nil {
		return newErrJoined(ErrDecode, decodeMessage, err).
			With(ErrorFieldSource, SourceBody)
	}
	if decoder.More() {
		return newErrJoined(ErrDecode, decodeMessage, errTrailingData).
			With(ErrorFieldSource, SourceBody)
	}
	return nil
}

// decodeMessage is the message of the errors returned by the decoding
// functions.
const decodeMessage = "unable to decode rest request"

// errTrailingData is the cause of the error of a body holding data after
// its JSON value.
var errTrailingData = errors.New("data after the JSON value")

// decode decodes the values into v with the decoder, failing with
// ErrDecode with the source in its context.
func decode(
	decoder *schema.Decoder,
	v any,
	values url.Values,
	source string,
) error {
	if err := decoder.Decode(v, values); err != nil {
		return newErrJoined(ErrDecode, decodeMessage, err).
			With(ErrorFieldSource, source)
	}
	return nil
}

// wildcards returns the names of the wildcards of the pattern, such as
// "id" and "path" for "GET /users/{id}/{path...}", leaving out "{$}".
func wildcards(
	pattern string,
) []string {
	var names []string
	for {
		_, rest, found := strings.Cut(pattern, "{")
		if !found {
			return names
		}
		name, after, _ := strings.Cut(rest, "}")
		if name = strings.TrimSuffix(name, "..."); name != "$" {
			names = append(names, name)
		}
		pattern = after
	}
}
