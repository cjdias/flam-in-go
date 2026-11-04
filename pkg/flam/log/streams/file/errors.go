package file

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the stream configuration, or its default,
	// does not hold a value of the expected type: a level, a list of
	// non-empty channels, and non-empty serializer and disk ids and path.
	ErrInvalidField = errors.New("invalid file log stream field")

	// ErrSerializer is the error wrapped, along with the error of the
	// [flam.ILogSerializerFactory], by the error returned from the creator
	// when the serializer cannot be taken from the factory.
	ErrSerializer = errors.New("unable to get file log stream serializer")

	// ErrDisk is the error wrapped, along with the error of the
	// [flam.IDiskFactory], or ErrMissingFactory, by the error returned
	// from the creator when the disk cannot be taken from the factory.
	ErrDisk = errors.New("unable to get file log stream disk")

	// ErrMissingFactory is the error wrapped, along with ErrDisk, by the
	// error returned from the creator when the container provides no
	// [flam.IDiskFactory].
	ErrMissingFactory = errors.New("factory not provided by the container")

	// ErrOpen is the error wrapped, along with the error of the disk, by
	// the error returned from the creator when the file cannot be opened.
	ErrOpen = errors.New("unable to open log file")
)

// sentinels are the sentinel errors of the driver, for the base helpers.
var sentinels = base.Sentinels{
	InvalidField:   ErrInvalidField,
	Serializer:     ErrSerializer,
	Disk:           ErrDisk,
	MissingFactory: ErrMissingFactory,
	Open:           ErrOpen,
}
