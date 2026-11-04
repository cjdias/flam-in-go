package tracing

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// NewBundle returns the [flam.IBundle] of the tracing feature, providing
// to the application container:
//   - a single [flam.ITracingExporterFactory], creating the exporters
//     configured at [ExporterConfigPath] with the creators of the
//     [ExporterCreatorGroup] group;
//   - a single [trace.TracerProvider]: the no-op one when
//     [EnabledConfigPath] holds false, or else the one of the
//     OpenTelemetry SDK, of the sampler and the resource of the
//     configuration, sending its spans to the exporters of
//     [ExportConfigPath];
//   - a single [propagation.TextMapPropagator], of the W3C trace context
//     and baggage, for the instrumentations reading or writing the
//     context of a trace across processes, such as the REST middleware of
//     [github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware].
//
// They are created on first use, reading the configuration once. It
// provides no exporter driver: register them with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters].
//
// The bundle implements [flam.IBootableBundle]: booting the application
// creates the tracer provider, failing when it cannot be created, and,
// unless [GlobalConfigPath] holds false, sets it and the propagator as
// the global ones of OpenTelemetry.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application shuts the tracer provider down, exporting the spans left
// within the timeout of [ShutdownTimeoutConfigPath], then closes the
// exporter factory, even when the shutdown fails, returning their errors
// joined. A global tracer provider shut down records no span anymore.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time checks that *bundleImpl implements IBootableBundle and
// IClosableBundle
var (
	_ flam.IBootableBundle = (*bundleImpl)(nil)
	_ flam.IClosableBundle = (*bundleImpl)(nil)
)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newExporterFactory).
		QueueProvider(newProvider).
		QueueProvider(newPropagator).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.setGlobal)
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.shutdownProvider),
		app.Invoke(b.closeExporterFactory),
	)
}

// setGlobal sets the tracer provider and the propagator as the global ones
// of OpenTelemetry, unless GlobalConfigPath holds false.
func (b *bundleImpl) setGlobal(
	config flam.IConfig,
	provider trace.TracerProvider,
	propagator propagation.TextMapPropagator,
) error {
	global, ok := config.Get(GlobalConfigPath, defaultGlobal).(bool)
	if !ok {
		return newErrField(bundleBootMessage, GlobalConfigPath)
	}
	if global {
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
	}
	return nil
}

// shutdownProvider shuts the tracer provider down, when it can be, within
// the timeout of ShutdownTimeoutConfigPath.
func (b *bundleImpl) shutdownProvider(
	config flam.IConfig,
	provider trace.TracerProvider,
) error {
	timeout, ok := settings.Duration(config.Get(ShutdownTimeoutConfigPath, defaultShutdownTimeout))
	if !ok {
		return newErrField(bundleCloseMessage, ShutdownTimeoutConfigPath)
	}
	shutdowner, ok := provider.(interface {
		Shutdown(ctx context.Context) error
	})
	if !ok {
		return nil
	}
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := shutdowner.Shutdown(ctx); err != nil {
		return newErrJoined(ErrShutdown, bundleCloseMessage, err)
	}
	return nil
}

func (b *bundleImpl) closeExporterFactory(
	f flam.ITracingExporterFactory,
) error {
	return f.Close()
}

// Messages of the errors returned by the bundle methods.
const (
	bundleBootMessage  = "unable to boot tracing"
	bundleCloseMessage = "unable to close tracing"
)
