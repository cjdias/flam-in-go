//go:build !windows && !plan9

package syslog

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the stream configuration, or its default,
	// does not hold a value of the expected type: a level, a list of
	// non-empty channels, a non-empty serializer id, a network, an address
	// and a tag, and a known facility.
	ErrInvalidField = errors.New("invalid syslog log stream field")

	// ErrSerializer is the error wrapped, along with the error of the
	// [flam.ILogSerializerFactory], by the error returned from the creator
	// when the serializer cannot be taken from the factory.
	ErrSerializer = errors.New("unable to get syslog log stream serializer")

	// ErrDial is the error wrapped, along with the error of log/syslog, by
	// the error returned from the creator when the daemon cannot be
	// connected to.
	ErrDial = errors.New("unable to connect to syslog")
)

// sentinels are the sentinel errors of the driver, for the base helpers.
var sentinels = base.Sentinels{
	InvalidField: ErrInvalidField,
	Serializer:   ErrSerializer,
}

// newErrDial returns the error wrapping ErrDial, joined with the error of
// log/syslog, with the message and the ErrorFieldID, ErrorFieldNetwork
// and ErrorFieldAddress context fields.
func newErrDial(
	message string,
	id string,
	network string,
	address string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrDial, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldNetwork, network).
		With(ErrorFieldAddress, address)
}
