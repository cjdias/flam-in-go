package logcontext

import (
	"context"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// newExtractor returns the extractor of the id and the principal of the
// request of a context, stored at the fields read from
// [RequestIdFieldConfigPath] and [PrincipalFieldConfigPath]. It is the
// provider registered by the bundle returned from [NewBundle].
func newExtractor(
	config flam.IConfig,
) flam.ILogContextExtractor {
	return &extractor{
		requestIdField: config.String(RequestIdFieldConfigPath, ContextFieldRequestId),
		principalField: config.String(PrincipalFieldConfigPath, ContextFieldPrincipal),
	}
}

// extractor is the [flam.ILogContextExtractor] of the id of the request
// and of its principal held by a context, each left out when the context
// holds none, or when its field is empty.
type extractor struct {
	requestIdField string
	principalField string
}

// compile time check that *extractor implements ILogContextExtractor
var _ flam.ILogContextExtractor = (*extractor)(nil)

func (e *extractor) Extract(
	ctx context.Context,
) flam.IBag {
	fields := container.Bag{}
	if id, ok := rest.RequestIdOf(ctx); ok && e.requestIdField != "" {
		fields[e.requestIdField] = id
	}
	if principal, ok := rest.PrincipalOf(ctx); ok && e.principalField != "" {
		fields[e.principalField] = principal
	}
	return fields
}
