package tracing

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The tracer provider and the propagator are optional, so that
// registering the driver does not require the tracing feature: their
// absence only fails the creation of an interceptor.
type creatorArgs struct {
	dig.In

	Provider   trace.TracerProvider          `optional:"true"`
	Propagator propagation.TextMapPropagator `optional:"true"`
}

// newCreator returns the [flam.IGrpcInterceptorCreator] of the tracing
// interceptors. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IGrpcInterceptorCreator {
	return &creator{
		provider:   args.Provider,
		propagator: args.Propagator,
	}
}

// creator is the [flam.IGrpcInterceptorCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the interceptor starting its spans with the tracer of
// [ScopeName] of the tracer provider, extracting the context of the calls
// from their metadata with the propagator.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IGrpcInterceptor, error) {
	id := bag.String(factory.ConfigFieldId)
	if c.provider == nil || c.propagator == nil {
		return nil, newErrCreate(ErrMissingTracing, creatorCreateMessage, id)
	}
	return &interceptor{
		id:         id,
		tracer:     c.provider.Tracer(ScopeName),
		propagator: c.propagator,
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create tracing grpc interceptor"
