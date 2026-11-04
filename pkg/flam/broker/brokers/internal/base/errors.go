package base

import (
	"context"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
)

// errorFieldField is the context field of the errors of the invalid
// fields, as the drivers document it.
const errorFieldField = "field"

// NewErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the id context field of the broker.
func NewErr(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(broker.ErrorFieldID, id)
}

// NewErrTopic returns the error of NewErr with the topic context field.
func NewErrTopic(
	sentinel error,
	message string,
	id string,
	topic string,
	cause error,
) *flam.Error {
	return NewErr(sentinel, message, id, cause).
		With(broker.ErrorFieldTopic, topic)
}

// NewErrInvalidField returns the error wrapping the sentinel of the
// invalid fields of a driver with the message and the id and field
// context fields.
func NewErrInvalidField(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	return NewErr(sentinel, message, id, nil).
		With(errorFieldField, field)
}

// ReceiveErr returns nil when the context is done, ending the subscription
// without error, or else the error, of the message, wrapping
// broker.ErrReceive, joined with the cause, with the id and topic context
// fields.
func ReceiveErr(
	ctx context.Context,
	message string,
	id string,
	topic string,
	cause error,
) error {
	if ctx.Err() != nil {
		return nil
	}
	return NewErrTopic(broker.ErrReceive, message, id, topic, cause)
}
