package jwt

import (
	"context"
	"net/http"
	"strings"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// ClaimsOf returns the claims of the token of the request held by the
// context, set by a jwt middleware, and whether it holds them.
func ClaimsOf(
	ctx context.Context,
) (gojwt.MapClaims, bool) {
	if ctx == nil {
		return nil, false
	}
	claims, ok := ctx.Value(claimsKey{}).(gojwt.MapClaims)
	return claims, ok
}

// middleware is the jwt [flam.IRestMiddleware], verifying the bearer
// tokens of the requests with its parser and key.
type middleware struct {
	id       string
	parser   *gojwt.Parser
	key      any
	optional bool
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next once their
// bearer token is verified, with its claims, and its subject as the
// principal, in their context, or answering them with a 401 envelope
// otherwise: of ErrMissingToken for a request without a bearer token,
// served without claims when the middleware is optional, or of
// ErrInvalidToken for a token failing verification, or holding a subject
// claim other than a string.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token, found := middlewareToken(req.Header)
		if !found {
			if m.optional {
				next.ServeHTTP(w, req)
				return
			}
			middlewareReject(w, req, ErrMissingToken, ChallengeMissing)
			return
		}

		claims := gojwt.MapClaims{}
		if _, err := m.parser.ParseWithClaims(token, claims, m.keyFunc); err != nil {
			middlewareReject(w, req, ErrInvalidToken, ChallengeInvalid)
			return
		}
		subject, err := claims.GetSubject()
		if err != nil {
			middlewareReject(w, req, ErrInvalidToken, ChallengeInvalid)
			return
		}

		ctx := context.WithValue(req.Context(), claimsKey{}, claims)
		if subject != "" {
			ctx = rest.WithPrincipal(ctx, subject)
		}
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// claimsKey is the key of the claims in the context of the requests.
type claimsKey struct{}

// keyFunc returns the key of the middleware, whatever the token, as the
// parser only accepts the algorithms of the middleware, all of which fit
// the key.
func (m *middleware) keyFunc(
	*gojwt.Token,
) (any, error) {
	return m.key, nil
}

// middlewareToken returns the token of the "Authorization: Bearer <token>"
// header, its scheme matched whatever its case, and whether the header
// holds a bearer one, maybe empty, which then fails verification.
func middlewareToken(
	header http.Header,
) (string, bool) {
	scheme, token, _ := strings.Cut(header.Get("Authorization"), " ")
	if !strings.EqualFold(scheme, middlewareScheme) {
		return "", false
	}
	return strings.TrimSpace(token), true
}

// middlewareReject answers the request with a 401 envelope of the error,
// along with the WWW-Authenticate header of the challenge.
func middlewareReject(
	w http.ResponseWriter,
	req *http.Request,
	e error,
	challenge string,
) {
	w.Header().Set("WWW-Authenticate", challenge)
	_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(http.StatusUnauthorized, e))
}

// middlewareScheme is the authentication scheme of the bearer tokens.
const middlewareScheme = "Bearer"
