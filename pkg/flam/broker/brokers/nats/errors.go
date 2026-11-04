package nats

import (
	"errors"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the broker configuration, or its default,
	// does not hold a value of the expected type and range, or when the
	// stream is missing with JetStream.
	ErrInvalidField = errors.New("invalid nats broker field")

	// ErrConnect is the error wrapped, along with the error of the client,
	// by the error returned from the creator when it cannot connect to the
	// server.
	ErrConnect = errors.New("unable to connect nats broker")

	// ErrStream is the error wrapped, along with the error of JetStream,
	// by the error returned from the creator when the stream cannot be
	// created or updated with its subjects.
	ErrStream = errors.New("unable to create nats broker stream")
)
