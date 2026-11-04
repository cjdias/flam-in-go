package standard

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// adaptor is the standard [flam.IGrpcLoggerAdaptor], signaling its reports
// on its channel of the logger.
type adaptor struct {
	id       string
	logger   flam.ILogger
	channel  string
	start    flam.LogLevel
	error    flam.LogLevel
	shutdown flam.LogLevel
	done     flam.LogLevel
}

func (a *adaptor) Id() string {
	return a.id
}

func (a *adaptor) Start(
	id string,
	address string,
) {
	a.signal(a.start, "grpc server started", id, container.Bag{ContextFieldAddress: address})
}

func (a *adaptor) Error(
	id string,
	e error,
) {
	ctx := container.Bag{}
	if e != nil {
		ctx[ContextFieldError] = e
	}
	a.signal(a.error, "grpc server failed", id, ctx)
}

func (a *adaptor) Shutdown(
	id string,
) {
	a.signal(a.shutdown, "grpc server shutting down", id, container.Bag{})
}

func (a *adaptor) Done(
	id string,
) {
	a.signal(a.done, "grpc server done", id, container.Bag{})
}

// signal signals the message at the level on the channel, with the server
// in the context, along with the fields of the report.
func (a *adaptor) signal(
	level flam.LogLevel,
	message string,
	id string,
	ctx container.Bag,
) {
	ctx[ContextFieldServer] = id
	a.logger.Signal(level, a.channel, message, ctx)
}
