package envelope

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// ContextFieldMessage is the field of the [Error] context holding the
// message of the error.
const ContextFieldMessage = "message"

// Error represents a structured error carried inside a response envelope.
type Error struct {
	// Status is the HTTP status the error answers with.
	Status int `json:"status"`
	// Id identifies where the error came from, as dot-separated parts
	// "s:<service>.e:<endpoint>.p:<param>.c:<code>". The service, endpoint and
	// param ids of 0 are left out, as is an error id never set, and a
	// non-numeric error id is written as it is, without "c:".
	Id string `json:"id"`
	// Context carries the error's details. "message" holds the error
	// message, when the error has one.
	Context container.Bag `json:"context"`

	serviceId  int
	endpointId int
	paramId    int
	errorId    string
}

// NewError creates a new envelope error from a status, error and optional
// context.
//
// The message of the error is stored at [ContextFieldMessage], then the
// context of the first [flam.Error] in its chain is merged in, then the
// given maps, in order, the later values replacing the earlier ones. As the
// context is sent in the response, the error should not carry details meant
// to stay internal. A nil error leaves the message out.
func NewError(
	status int,
	e error,
	ctx ...map[string]any,
) Error {
	err := Error{
		Status:  status,
		Context: container.Bag{},
	}

	if !value.IsNil(e) {
		err.Context[ContextFieldMessage] = e.Error()

		var flamErr *flam.Error
		if errors.As(e, &flamErr) {
			maps.Copy(err.Context, flamErr.Context())
		}
	}

	for _, c := range ctx {
		maps.Copy(err.Context, c)
	}

	return err
}

// Error returns the error's message: the "message" context entry, written
// out as text when it is not a string, or the status text when there is
// none.
func (e Error) Error() string {
	switch message := e.Context[ContextFieldMessage].(type) {
	case string:
		return message
	case nil:
		return http.StatusText(e.Status)
	default:
		return fmt.Sprint(message)
	}
}

// WithStatus returns a copy of the envelope error with the given status set.
func (e Error) WithStatus(
	val int,
) Error {
	e.Status = val
	return e
}

// WithServiceId returns a copy of the envelope error with the given service
// id set.
func (e Error) WithServiceId(
	val int,
) Error {
	e.serviceId = val
	return e.compose()
}

// WithEndpointId returns a copy of the envelope error with the given
// endpoint id set.
func (e Error) WithEndpointId(
	val int,
) Error {
	e.endpointId = val
	return e.compose()
}

// WithParamId returns a copy of the envelope error with the given param id
// set.
func (e Error) WithParamId(
	val int,
) Error {
	e.paramId = val
	return e.compose()
}

// WithErrorId returns a copy of the envelope error with the given error id
// set, written out as text.
func (e Error) WithErrorId(
	val any,
) Error {
	e.errorId = fmt.Sprint(val)
	return e.compose()
}

// WithContext returns a copy of the envelope error with the given context
// entry set.
func (e Error) WithContext(
	key string,
	val any,
) Error {
	// the context is a map, so the copy would otherwise share it with the
	// original
	context := make(container.Bag, len(e.Context)+1)
	maps.Copy(context, e.Context)
	context[key] = val
	e.Context = context
	return e
}

// compose returns the error with its Id composed from its ids.
func (e Error) compose() Error {
	parts := make([]string, 0, 4)
	if e.serviceId != 0 {
		parts = append(parts, "s:"+strconv.Itoa(e.serviceId))
	}
	if e.endpointId != 0 {
		parts = append(parts, "e:"+strconv.Itoa(e.endpointId))
	}
	if e.paramId != 0 {
		parts = append(parts, "p:"+strconv.Itoa(e.paramId))
	}
	if e.errorId != "" {
		if code, err := strconv.Atoi(e.errorId); err == nil {
			parts = append(parts, "c:"+strconv.Itoa(code))
		} else {
			parts = append(parts, e.errorId)
		}
	}
	e.Id = strings.Join(parts, ".")
	return e
}
