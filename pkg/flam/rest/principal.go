package rest

import (
	"context"
)

// WithPrincipal returns a copy of the context holding the principal, the
// name of the client a middleware authenticated the request of, such as
// the subject of a token, read by [PrincipalOf]. It lets the other
// middlewares, such as a rate limiter, and the handlers tell the clients
// apart without depending on the middleware authenticating them. An empty
// principal stands for none.
func WithPrincipal(
	ctx context.Context,
	principal string,
) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalOf returns the principal held by the context, set by
// [WithPrincipal], and whether it holds a non-empty one.
func PrincipalOf(
	ctx context.Context,
) (string, bool) {
	if ctx == nil {
		return "", false
	}
	principal, _ := ctx.Value(principalKey{}).(string)
	return principal, principal != ""
}

// principalKey is the key of the principal in the context of the requests.
type principalKey struct{}
