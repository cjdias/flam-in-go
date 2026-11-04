package database

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// connection creator when a field of the connection configuration, or
	// its default, does not hold a non-empty id.
	ErrInvalidField = errors.New("invalid database connection field")

	// ErrDialect is the error wrapped, along with the error of the
	// [flam.IDatabaseDialectFactory], by the error returned from the
	// connection creator when the dialect cannot be taken from the
	// factory.
	ErrDialect = errors.New("unable to get database connection dialect")

	// ErrConfig is the error wrapped, along with the error of the
	// [flam.IDatabaseConfigFactory], by the error returned from the
	// connection creator when the gorm configuration cannot be taken from
	// the factory.
	ErrConfig = errors.New("unable to get database connection config")

	// ErrOpen is the error wrapped, along with the error of gorm, by the
	// error returned from the connection creator when the connection
	// cannot be opened.
	ErrOpen = errors.New("unable to open database connection")

	// ErrResolver is the error wrapped, along with the error of gorm, by
	// the error returned from the connection creator when the read
	// replicas or the extra sources of the connection cannot be opened.
	ErrResolver = errors.New("unable to open database connection replicas")

	// ErrNilClock is the error wrapped by the error returned from the
	// connection creator when the connection is to be retried without an
	// [flam.IClock] in the container, and from [Retry] when its options
	// have no clock.
	ErrNilClock = errors.New("nil clock")

	// ErrNilRetryFunc is the error wrapped by the error returned from
	// [Retry] when the function to retry is nil.
	ErrNilRetryFunc = errors.New("nil function to retry")

	// ErrRetry is the error wrapped, along with the error of the last
	// attempt, by the error returned from [Retry] when the attempts are
	// exhausted, and, along with the error of the context, when the
	// context ends before an attempt, the first one included.
	ErrRetry = errors.New("unable to complete with retries")

	// ErrNilDialector is the error wrapped by the error returned from
	// [NewDialect] when the dialector is nil.
	ErrNilDialector = errors.New("nil database dialector")

	// ErrNilDB is the error wrapped by the error returned from
	// [NewConnection] when the *gorm.DB is nil.
	ErrNilDB = errors.New("nil gorm database")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrDialect returns the error wrapping ErrDialect, joined with the
// error of the dialect factory, with the message and the ErrorFieldID and
// ErrorFieldDialect context fields.
func newErrDialect(
	message string,
	id string,
	dialect string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrDialect, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldDialect, dialect)
}

// newErrConfig returns the error wrapping ErrConfig, joined with the error
// of the configuration factory, with the message and the ErrorFieldID and
// ErrorFieldConfig context fields.
func newErrConfig(
	message string,
	id string,
	config string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrConfig, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldConfig, config)
}

// newErrOpen returns the error wrapping ErrOpen, joined with the error of
// gorm, with the message and the ErrorFieldID, ErrorFieldDialect and
// ErrorFieldConfig context fields.
func newErrOpen(
	message string,
	id string,
	dialect string,
	config string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrOpen, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldDialect, dialect).
		With(ErrorFieldConfig, config)
}

// newErrResolver returns the error wrapping ErrResolver, joined with the
// error of gorm, with the message and the ErrorFieldID context field.
func newErrResolver(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrResolver, err), message).
		With(ErrorFieldID, id)
}

// newErrRetry returns the error wrapping ErrRetry, joined with the errors,
// with the ErrorFieldAttempts context field.
func newErrRetry(
	attempts int,
	errs ...error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRetry, errors.Join(errs...)), retryMessage).
		With(ErrorFieldAttempts, attempts)
}

// newErrNil returns the error wrapping the sentinel of a nil argument with
// the message and the ErrorFieldID context field.
func newErrNil(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrClose returns the error wrapping the error of the closing of the
// pool of a connection with the message and the ErrorFieldID context
// field.
func newErrClose(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}

// newErrRetryNil returns the error wrapping the sentinel of a nil argument
// of Retry.
func newErrRetryNil(
	sentinel error,
) *flam.Error {
	return flam.NewWrappedError(sentinel, retryMessage)
}
