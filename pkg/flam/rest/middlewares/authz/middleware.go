package authz

import (
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/jwt"
)

// middleware is the authz [flam.IRestMiddleware], authorizing the requests
// of each endpoint by the first of its rules matching it, or else by its
// policy.
type middleware struct {
	id         string
	deny       bool
	rules      []rule
	rolesClaim []string
	principals map[string][]string
	challenge  string
}

// rule is a rule of a middleware: the ids and the patterns of the
// endpoints it matches, and the scopes and the roles their requests
// require, or whether they are public.
type rule struct {
	endpoints []int
	patterns  []string
	scopes    []string
	roles     []string
	public    bool
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns next for an endpoint of a public rule, or else the handler
// serving the requests through next once authorized by the rule of the
// endpoint, or by the policy when no rule matches it: answering with a
// 403 envelope of ErrForbidden every request when the policy denies the
// endpoint, or else with a 401 envelope of ErrUnauthenticated a request
// without claims or principal, or with a 403 envelope of ErrForbidden a
// request without every scope, or one of the roles, of the rule.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	r, found := m.rule(endpoint)
	switch {
	case !found && m.deny:
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			middlewareReject(w, req, http.StatusForbidden, ErrForbidden)
		})
	case r.public:
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		claims, hasClaims := jwt.ClaimsOf(req.Context())
		principal, hasPrincipal := rest.PrincipalOf(req.Context())
		if !hasClaims && !hasPrincipal {
			if m.challenge != "" {
				w.Header().Set("WWW-Authenticate", m.challenge)
			}
			middlewareReject(w, req, http.StatusUnauthorized, ErrUnauthenticated)
			return
		}
		if !m.allows(r, claims, principal) {
			middlewareReject(w, req, http.StatusForbidden, ErrForbidden)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// rule returns the first rule matching the endpoint, by its id or its
// pattern, and whether one does.
func (m *middleware) rule(
	endpoint flam.RestEndpoint,
) (rule, bool) {
	for _, r := range m.rules {
		if slices.Contains(r.endpoints, endpoint.Id) {
			return r, true
		}
		for _, pattern := range r.patterns {
			if matched, _ := path.Match(pattern, endpoint.Pattern); matched {
				return r, true
			}
		}
	}
	return rule{}, false
}

// allows reports whether the claims and the principal hold every scope of
// the rule, and one of its roles, when it requires any.
func (m *middleware) allows(
	r rule,
	claims map[string]any,
	principal string,
) bool {
	if len(r.scopes) > 0 {
		scopes := append(middlewareList(claims["scope"]), middlewareList(claims["scp"])...)
		for _, scope := range r.scopes {
			if !slices.Contains(scopes, scope) {
				return false
			}
		}
	}
	if len(r.roles) == 0 {
		return true
	}
	roles := append(middlewareList(middlewareClaim(claims, m.rolesClaim)), m.principals[principal]...)
	for _, role := range r.roles {
		if slices.Contains(roles, role) {
			return true
		}
	}
	return false
}

// middlewareClaim returns the value of the claims at the path of the
// segments, walking down the nested objects, or nil when the path is
// empty or missing.
func middlewareClaim(
	claims map[string]any,
	segments []string,
) any {
	if len(segments) == 0 {
		return nil
	}
	var value any = claims
	for _, segment := range segments {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[segment]
	}
	return value
}

// middlewareList returns the strings of the claim value, the space
// separated items of a string, or the strings of a list, its other items
// being ignored, or nil for a value of another type.
func middlewareList(
	value any,
) []string {
	switch v := value.(type) {
	case string:
		return strings.Fields(v)
	case []any:
		var items []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
		return items
	}
	return nil
}

// middlewareReject answers the request with an envelope of the status and
// the error.
func middlewareReject(
	w http.ResponseWriter,
	req *http.Request,
	status int,
	e error,
) {
	_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(status, e))
}
