package base

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
)

// Handle calls the handler with the message, returning its error, or,
// when it panics, the error wrapping broker.ErrHandlerPanic, joined with
// the panic value when it is an error, with the id, topic, message, panic
// and stack context fields, so that the subscription takes the message as
// failed instead of crashing the program.
func Handle(
	ctx context.Context,
	id string,
	handler flam.BrokerHandler,
	msg flam.IBrokerMessage,
) (err error) {
	defer func() {
		if value := recover(); value != nil {
			cause := broker.ErrHandlerPanic
			if panicErr, isErr := value.(error); isErr {
				cause = fmt.Errorf("%w: %w", broker.ErrHandlerPanic, panicErr)
			}
			err = NewErrTopic(cause, handleMessage, id, msg.Topic(), nil).
				With(broker.ErrorFieldMessage, msg.Id()).
				With(broker.ErrorFieldPanic, value).
				With(broker.ErrorFieldStack, string(debug.Stack()))
		}
	}()
	return handler(ctx, msg)
}

// handleMessage is the message of the errors of the panicking handlers.
const handleMessage = "unable to handle broker message"
