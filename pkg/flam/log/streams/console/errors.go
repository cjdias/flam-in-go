package console

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the stream configuration, or its default,
	// does not hold a value of the expected type: a level, a list of
	// non-empty channels, a non-empty serializer id, and an output among
	// OutputStdout and OutputStderr.
	ErrInvalidField = errors.New("invalid console log stream field")

	// ErrSerializer is the error wrapped, along with the error of the
	// [flam.ILogSerializerFactory], by the error returned from the creator
	// when the serializer cannot be taken from the factory.
	ErrSerializer = errors.New("unable to get console log stream serializer")
)

// sentinels are the sentinel errors of the driver, for the base helpers.
var sentinels = base.Sentinels{
	InvalidField: ErrInvalidField,
	Serializer:   ErrSerializer,
}
