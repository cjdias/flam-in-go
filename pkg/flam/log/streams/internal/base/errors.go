package base

import (
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Sentinels are the sentinel errors of a driver, wrapped by the errors of
// its creator: the drivers without disk leave Disk, MissingFactory and
// Open nil.
type Sentinels struct {
	InvalidField   error
	Serializer     error
	Disk           error
	MissingFactory error
	Open           error
}

// Context fields of the errors of the creators, as the drivers document
// them.
const (
	errorFieldID         = "id"
	errorFieldField      = "field"
	errorFieldSerializer = "serializer"
	errorFieldDisk       = "disk"
	errorFieldPath       = "path"
)

// NewErrInvalidField returns the error wrapping the InvalidField sentinel
// with the message and the id and field context fields.
func NewErrInvalidField(
	sentinels Sentinels,
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(sentinels.InvalidField, message).
		With(errorFieldID, id).
		With(errorFieldField, field)
}

// NewErrOpen returns the error wrapping the Open sentinel, joined with the
// error of the disk, with the message and the id, disk and path context
// fields.
func NewErrOpen(
	sentinels Sentinels,
	message string,
	id string,
	disk string,
	path string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinels.Open, err), message).
		With(errorFieldID, id).
		With(errorFieldDisk, disk).
		With(errorFieldPath, path)
}

// Serializer returns the serializer of the id taken from the factory, or
// the error wrapping the Serializer sentinel, joined with the error of the
// factory, with the message and the id and serializer context fields.
func Serializer(
	serializers flam.ILogSerializerFactory,
	sentinels Sentinels,
	message string,
	id string,
	serializerID string,
) (flam.ILogSerializer, error) {
	serializer, err := serializers.Get(serializerID)
	if err != nil {
		return nil, flam.NewWrappedError(fmt.Errorf("%w: %w", sentinels.Serializer, err), message).
			With(errorFieldID, id).
			With(errorFieldSerializer, serializerID)
	}
	return serializer, nil
}

// Disk returns the disk of the id taken from the factory, which may be
// nil when the container provides none, or the error wrapping the Disk
// sentinel, joined with the error of the factory or with the
// MissingFactory sentinel, with the message and the id and disk context
// fields.
func Disk(
	disks flam.IDiskFactory,
	sentinels Sentinels,
	message string,
	id string,
	diskID string,
) (flam.IDisk, error) {
	err := sentinels.MissingFactory
	if disks != nil {
		var disk flam.IDisk
		if disk, err = disks.Get(diskID); err == nil {
			return disk, nil
		}
	}
	return nil, flam.NewWrappedError(fmt.Errorf("%w: %w", sentinels.Disk, err), message).
		With(errorFieldID, id).
		With(errorFieldDisk, diskID)
}
