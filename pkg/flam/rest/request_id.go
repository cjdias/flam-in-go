package rest

import (
	"context"
)

// WithRequestId returns a copy of the context holding the id of the
// request, read by [RequestIdOf], as stored by the request id middleware
// of [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/requestid],
// so that the handlers and the other middlewares need not import it. A
// nil context stands for [context.Background].
func WithRequestId(
	ctx context.Context,
	id string,
) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIdKey{}, id)
}

// RequestIdOf returns the id of the request held by the context, stored by
// [WithRequestId], and whether it holds one.
func RequestIdOf(
	ctx context.Context,
) (string, bool) {
	if ctx == nil {
		return "", false
	}
	id, ok := ctx.Value(requestIdKey{}).(string)
	return id, ok
}

// requestIdKey is the key of the request id in the context of the
// requests.
type requestIdKey struct{}
