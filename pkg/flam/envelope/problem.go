package envelope

import (
	"encoding/json"
	"maps"
	"net/http"
	"strconv"

	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// ProblemTypeBase is the base of the type of the problems returned by
// [NewProblem], followed by their status, such as
// "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404".
const ProblemTypeBase = "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/"

// ContentTypeProblem is the content type of the JSON of a [Problem].
const ContentTypeProblem = "application/problem+json"

// Problem is the problem details of an error response, as described by
// RFC 9457, written as JSON with the members of its fields, those left
// empty being left out, along with its extensions.
type Problem struct {
	// Type identifies the type of the problem, as a URI.
	Type string
	// Title is a short summary of the type of the problem.
	Title string
	// Status is the HTTP status of the response.
	Status int
	// Detail explains the occurrence of the problem.
	Detail string
	// Instance identifies the occurrence of the problem, as a URI.
	Instance string
	// Extensions are the other members of the problem, such as "id", which
	// do not replace the members of the fields.
	Extensions container.Bag
}

// NewProblem returns the problem of the first error of the envelope: its
// status, or 500 when out of the 400 to 599 range of the error statuses,
// as a response of a status under 200 is not a final one, its message,
// when it has one, as the detail, and its id, when set, and the other
// entries of its context as extensions,
// along with every error of the envelope at "errors" when it holds more
// than one. The type is [ProblemTypeBase] followed by the status, and the
// title the text of the status. An envelope with no error is a problem of
// status 500.
func NewProblem(
	env Envelope,
) Problem {
	problem := Problem{
		Status:     http.StatusInternalServerError,
		Extensions: container.Bag{},
	}

	if len(env.Errors) > 0 {
		e := env.Errors[0]
		maps.Copy(problem.Extensions, e.Context)
		delete(problem.Extensions, ContextFieldMessage)
		if e.Id != "" {
			problem.Extensions["id"] = e.Id
		}
		if e.Status >= 400 && e.Status <= 599 {
			problem.Status = e.Status
		}
		if e.Context[ContextFieldMessage] != nil {
			problem.Detail = e.Error()
		}
	}
	if len(env.Errors) > 1 {
		problem.Extensions["errors"] = env.Errors
	}

	problem.Type = ProblemTypeBase + strconv.Itoa(problem.Status)
	problem.Title = http.StatusText(problem.Status)
	return problem
}

// MarshalJSON returns the JSON object of the problem: its extensions, and
// the members of its fields not left empty, replacing the extensions of
// the same name.
func (p Problem) MarshalJSON() ([]byte, error) {
	members := make(map[string]any, len(p.Extensions)+5)
	maps.Copy(members, p.Extensions)
	for name, value := range map[string]string{
		"type":     p.Type,
		"title":    p.Title,
		"detail":   p.Detail,
		"instance": p.Instance,
	} {
		if value != "" {
			members[name] = value
		}
	}
	if p.Status != 0 {
		members["status"] = p.Status
	}
	return json.Marshal(members)
}
