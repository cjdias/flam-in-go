package outbox

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilTx is the error wrapped by the error returned from [Add] and
	// [AddJSON] when the transaction is nil.
	ErrNilTx = errors.New("nil outbox transaction")

	// ErrInvalidTopic is the error wrapped by the error returned from
	// [Add] and [AddJSON] when the topic is empty.
	ErrInvalidTopic = errors.New("invalid outbox topic")

	// ErrEncode is the error wrapped, along with the error of the encoder,
	// by the error returned from [AddJSON] when the value cannot be
	// encoded.
	ErrEncode = errors.New("unable to encode outbox message")

	// ErrAdd is the error wrapped, along with the error of gorm, by the
	// error returned from [Add] and [AddJSON] when the message cannot be
	// stored.
	ErrAdd = errors.New("unable to add outbox message")

	// ErrMigrate is the error wrapped, along with the error of gorm, by
	// the error of the creation of a relay, or of [Migrate], when the
	// [Table] table cannot be created or updated.
	ErrMigrate = errors.New("unable to migrate outbox table")

	// ErrInvalidField is the error wrapped by the error of the creation of
	// a relay when its configuration is not a bag, or when a field of it,
	// or its default, does not hold a value of the expected type and
	// range.
	ErrInvalidField = errors.New("invalid outbox relay field")

	// ErrConnection is the error wrapped, along with the error of the
	// [flam.IDatabaseConnectionFactory], by the error of the creation of a
	// relay when its database connection cannot be taken from the factory,
	// or when the container provides no factory.
	ErrConnection = errors.New("unable to get outbox relay connection")

	// ErrBroker is the error wrapped, along with the error of the
	// [flam.IBrokerFactory], by the error of the creation of a relay when
	// its broker cannot be taken from the factory, or when the container
	// provides no factory.
	ErrBroker = errors.New("unable to get outbox relay broker")

	// ErrRunning is the error wrapped by the error returned from the Run
	// method of a relay when it runs already.
	ErrRunning = errors.New("outbox relay running")

	// ErrClaim is the error wrapped, along with the error of gorm, by the
	// error a relay logs when it cannot claim the messages, polling again
	// later.
	ErrClaim = errors.New("unable to claim outbox messages")

	// ErrDecode is the error wrapped, along with the error of the decoder,
	// by the error a relay logs when the headers of a message cannot be
	// decoded, the message being retried as when the broker refuses it.
	ErrDecode = errors.New("unable to decode outbox message headers")

	// ErrPublish is the error wrapped, along with the error of the broker,
	// by the error a relay logs when the broker refuses a message, which
	// is retried after a backoff.
	ErrPublish = errors.New("unable to publish outbox message")

	// ErrRecord is the error wrapped, along with the error of gorm, by the
	// error a relay logs when it cannot record the outcome of a message,
	// which is published again once its lease expires.
	ErrRecord = errors.New("unable to record outbox message outcome")

	// ErrLeaseLost is the error wrapped by the error a relay logs when the
	// message it published was claimed by another relay, as its lease
	// expired.
	ErrLeaseLost = errors.New("outbox message lease lost")

	// ErrCleanup is the error wrapped, along with the error of gorm, by
	// the error a relay logs when it cannot delete the sent messages past
	// their retention.
	ErrCleanup = errors.New("unable to clean up outbox messages")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message.
func newErr(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message)
}

// newErrRelay returns the error of newErr with the ErrorFieldID context
// field of the relay.
func newErrRelay(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	return newErr(sentinel, message, cause).
		With(ErrorFieldID, id)
}

// newErrMessage returns the error of newErrRelay with the
// ErrorFieldMessage and ErrorFieldTopic context fields of the message.
func newErrMessage(
	sentinel error,
	message string,
	id string,
	row *Row,
	cause error,
) *flam.Error {
	return newErrRelay(sentinel, message, id, cause).
		With(ErrorFieldMessage, row.ID).
		With(ErrorFieldTopic, row.Topic)
}
