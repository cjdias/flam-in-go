package envelope

import "slices"

// Envelope is the standard response wrapper carrying data, filters and errors.
type Envelope struct {
	// Errors are the errors the response reports. When there are any, the first
	// one's status is the response status.
	Errors []Error `json:"errors,omitempty"`
	// Data is the response payload.
	Data any `json:"data,omitempty"`
	// Filter describes how Data was selected, such as the pagination set by
	// WithPagination.
	Filter any `json:"filter,omitempty"`
}

// NewEnvelope creates a new response envelope with optional data.
func NewEnvelope(
	data ...any,
) Envelope {
	env := Envelope{}
	if len(data) > 0 {
		env.Data = data[0]
	}
	return env
}

// WithNewError returns a copy of the envelope with the error created by
// [NewError] appended.
func (env Envelope) WithNewError(
	status int,
	e error,
	ctx ...map[string]any,
) Envelope {
	return env.WithError(NewError(status, e, ctx...))
}

// WithError returns a copy of the envelope with the given error appended.
func (env Envelope) WithError(
	envelopeError Error,
) Envelope {
	// clipped, so that the append always allocates: appending in place could
	// overwrite an error another copy of the envelope appended to the same
	// backing array
	env.Errors = append(slices.Clip(env.Errors), envelopeError)
	return env
}

// WithServiceId returns a copy of the envelope with the service id set on all errors.
func (env Envelope) WithServiceId(
	val int,
) Envelope {
	env.Errors = slices.Clone(env.Errors)
	for id, e := range env.Errors {
		env.Errors[id] = e.WithServiceId(val)
	}
	return env
}

// WithEndpointId returns a copy of the envelope with the endpoint id set on all errors.
func (env Envelope) WithEndpointId(
	val int,
) Envelope {
	env.Errors = slices.Clone(env.Errors)
	for id, e := range env.Errors {
		env.Errors[id] = e.WithEndpointId(val)
	}
	return env
}

// WithPagination returns a copy of the envelope with pagination metadata set as
// its filter. An optional path makes the emitted prev/next links resolvable on
// their own rather than relative to the request that produced them.
func (env Envelope) WithPagination(
	search string,
	start,
	count,
	total int64,
	path ...string,
) Envelope {
	env.Filter = newPagination(search, start, count, total, path...)
	return env
}
