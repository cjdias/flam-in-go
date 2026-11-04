package log

import (
	"errors"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the log feature, providing to
// the application container a single logger, as both its [flam.ILogger]
// and its [flam.IContextLogger], and the
// [flam.ILogSerializerFactory] and the [flam.ILogStreamFactory], created
// on first use from the configuration at [SerializerConfigPath] and
// [StreamConfigPath] and the creators of the [SerializerCreatorGroup] and
// [StreamCreatorGroup] groups. It provides no serializer nor stream
// driver: register it with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/log/serializers] and
// [github.com/cjdias/flam-in-go/pkg/flam/log/streams].
//
// The logger stamps its entries with the [flam.IClock] of the container
// and keeps them until flushed, then writes each of them to every stream
// stored in the stream factory, which filters them by level and channel.
// To the entries given with a context, it adds the fields extracted from
// it by the [flam.ILogContextExtractor] values of the
// [ContextExtractorGroup] group, the contexts given along winning over
// them, while the winner between extractors setting the same field is
// unspecified, as the container does not order its groups. The
// bundle provides in the group the extractor of the ids of the
// OpenTelemetry trace and span of the context, at the fields read from
// [TraceIdFieldConfigPath] and [SpanIdFieldConfigPath] when the logger is
// created.
//
// The bundle implements [flam.IBootableBundle]: when the application
// boots, it creates every stream configured at [StreamConfigPath], in id
// order, with the stream factory, unless [BootConfigPath] holds false. The
// first stream failing to be created fails the boot.
//
// It then starts flushing the logger at the period stored at
// [FlusherPeriodConfigPath], one second by default, with a recurring
// trigger of the [flam.ITriggerFactory] of the container, and restarts the
// trigger when the period changes. The trigger factory, provided by the
// bundle of [github.com/cjdias/flam-in-go/pkg/flam/trigger], is only
// required while the period is positive: the boot fails with
// ErrMissingTriggerFactory otherwise. Where nobody waits for them, a
// periodic flush failing, and a change of the period failing to restart
// the trigger, which leaves the logger without periodic flush until the
// period changes again, are broadcast as error entries, with the error at
// [ErrorFieldError], written by the next flush.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application stops the flushing and flushes the logger a last time, a
// periodic flush fired before but run afterwards flushing nothing, then
// closes the stream factory, then the serializer factory, even when one
// fails, returning their errors joined. As the flusher and the factories
// are taken from the container, closing an application that never used
// them creates them first, and fails when they cannot be created.
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
		QueueProvider(newSerializerFactory).
		QueueProvider(newStreamFactory).
		QueueProvider(newLogger, dig.As(new(flam.ILogger), new(flam.IContextLogger))).
		QueueProvider(newTraceExtractor, dig.Group(ContextExtractorGroup)).
		QueueProvider(newBooter).
		QueueProvider(newFlusher).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.boot)
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeFlusher),
		app.Invoke(b.closeStreamFactory),
		app.Invoke(b.closeSerializerFactory),
	)
}

// boot creates the configured streams, then starts the flusher.
func (b *bundleImpl) boot(
	booter *booter,
	flusher *flusher,
) error {
	if err := booter.Boot(); err != nil {
		return err
	}
	return flusher.Boot()
}

func (b *bundleImpl) closeFlusher(
	f *flusher,
) error {
	return f.Close()
}

func (b *bundleImpl) closeStreamFactory(
	f flam.ILogStreamFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeSerializerFactory(
	f flam.ILogSerializerFactory,
) error {
	return f.Close()
}
