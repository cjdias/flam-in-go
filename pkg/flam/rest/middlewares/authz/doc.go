// Package authz is the REST middleware driver authorizing the requests of
// each endpoint by the claims of their token, set by the jwt middlewares
// and read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/jwt.ClaimsOf],
// or by their principal, set by the authentication middlewares, such as
// the apikey ones, and read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.PrincipalOf]: list it after
// them.
//
// The rules of [ConfigFieldRules] tell the endpoints they match, by their
// id, [RuleFieldEndpoints], whatever their service, or by their pattern,
// [RuleFieldPatterns], such as "GET /admin/*", and what their requests
// require: every scope of [RuleFieldScopes], held by the "scope" claim, a
// space separated string, or the "scp" claim, a list or a string, and one
// of the roles of [RuleFieldRoles], held by the claim of
// [ConfigFieldRolesClaim], such as "roles" or "realm_access.roles", or
// given to the principal by [ConfigFieldPrincipals], such as for the
// principals of API keys. A rule requiring neither serves every
// authenticated request, and a rule of [RuleFieldPublic] serves every
// request, authenticated or not. The first rule matching an endpoint
// applies to it, chosen once, when the endpoint is wrapped, and the
// policy of [ConfigFieldPolicy] to the endpoints none matches:
// [PolicyDeny], the default, failing closed, or [PolicyAuthenticated].
//
// A request is authenticated when its context holds claims or a
// principal. A request of a non-public endpoint without authentication is
// answered with a 401 envelope of [ErrUnauthenticated], along with the
// WWW-Authenticate header of [ConfigFieldChallenge], "Bearer" by default,
// and a request of an endpoint denied by the policy, or without the
// scopes or roles of its rule, with a 403 envelope of [ErrForbidden],
// neither telling the client what it lacks.
package authz
