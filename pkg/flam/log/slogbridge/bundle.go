package slogbridge

import (
	"log/slog"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] providing to the application
// container the [log/slog.Logger] returned from [NewLogger] for the
// [flam.ILogger] of the container, provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/log], with the channel stored at
// [ChannelConfigPath], the minimum level stored at [LevelConfigPath] and
// the time field stored at [TimeFieldConfigPath], read when the logger is
// created.
//
// The bundle implements [flam.IBootableBundle]: when the application
// boots, it makes the logger the default logger of [log/slog], with
// [log/slog.SetDefault], when [SetDefaultConfigPath] holds true. The
// previous default logger is not restored when the application closes.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IBootableBundle
var _ flam.IBootableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newSlogLogger).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.boot)
}

// boot makes the logger the default logger of [log/slog] when
// SetDefaultConfigPath holds true.
func (b *bundleImpl) boot(
	config flam.IConfig,
	logger *slog.Logger,
) {
	if config.Bool(SetDefaultConfigPath, defaultSetDefault) {
		slog.SetDefault(logger)
	}
}

// newSlogLogger returns the [log/slog.Logger] of the logger, configured
// at ChannelConfigPath, LevelConfigPath and TimeFieldConfigPath. It is the
// provider registered by the bundle.
func newSlogLogger(
	config flam.IConfig,
	logger flam.ILogger,
) *slog.Logger {
	return NewLogger(logger, &HandlerOptions{
		Channel:   config.String(ChannelConfigPath, DefaultChannel),
		Level:     SlogLevel(flam.LogNone.From(config.Get(LevelConfigPath), flam.LogInfo)),
		TimeField: config.String(TimeFieldConfigPath),
	})
}
