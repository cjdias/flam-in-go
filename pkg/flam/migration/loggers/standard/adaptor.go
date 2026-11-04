package standard

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// adaptor is the standard [flam.IMigrationLoggerAdaptor], signaling its
// reports on its channel of the logger.
type adaptor struct {
	id      string
	logger  flam.ILogger
	channel string
	start   flam.LogLevel
	error   flam.LogLevel
	done    flam.LogLevel
}

func (a *adaptor) Id() string {
	return a.id
}

func (a *adaptor) UpStart(
	info flam.MigrationInfo,
) {
	a.signal(a.start, "migration up started", info, nil)
}

func (a *adaptor) UpError(
	info flam.MigrationInfo,
	e error,
) {
	a.signal(a.error, "migration up failed", info, e)
}

func (a *adaptor) UpDone(
	info flam.MigrationInfo,
) {
	a.signal(a.done, "migration up done", info, nil)
}

func (a *adaptor) DownStart(
	info flam.MigrationInfo,
) {
	a.signal(a.start, "migration down started", info, nil)
}

func (a *adaptor) DownError(
	info flam.MigrationInfo,
	e error,
) {
	a.signal(a.error, "migration down failed", info, e)
}

func (a *adaptor) DownDone(
	info flam.MigrationInfo,
) {
	a.signal(a.done, "migration down done", info, nil)
}

// signal signals the message at the level on the channel, with the
// migration, and the error when not nil, in the context.
func (a *adaptor) signal(
	level flam.LogLevel,
	message string,
	info flam.MigrationInfo,
	e error,
) {
	ctx := container.Bag{
		ContextFieldMigration:   info.Id,
		ContextFieldDescription: info.Description,
	}
	if e != nil {
		ctx[ContextFieldError] = e
	}
	a.logger.Signal(level, a.channel, message, ctx)
}
