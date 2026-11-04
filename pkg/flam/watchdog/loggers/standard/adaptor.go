package standard

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// adaptor is the standard [flam.IWatchdogLoggerAdaptor], signaling its
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

func (a *adaptor) Start(
	id string,
) {
	a.signal(a.start, "watchdog process started", id, nil)
}

func (a *adaptor) Error(
	id string,
	e error,
) {
	a.signal(a.error, "watchdog process failed", id, e)
}

func (a *adaptor) Done(
	id string,
) {
	a.signal(a.done, "watchdog process done", id, nil)
}

// signal signals the message at the level on the channel, with the
// process, and the error when not nil, in the context, along with the
// value and the stack of a panic.
func (a *adaptor) signal(
	level flam.LogLevel,
	message string,
	id string,
	e error,
) {
	ctx := container.Bag{ContextFieldProcess: id}
	if e != nil {
		ctx[ContextFieldError] = e
	}
	var fe *flam.Error
	if errors.Is(e, watchdog.ErrPanic) && errors.As(e, &fe) {
		errCtx := fe.Context()
		ctx[ContextFieldPanic] = errCtx[watchdog.ErrorFieldPanic]
		ctx[ContextFieldStack] = errCtx[watchdog.ErrorFieldStack]
	}
	a.logger.Signal(level, a.channel, message, ctx)
}
