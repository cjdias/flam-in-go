package disk

import (
	"context"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the disk factory only when provided.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Disks  flam.IDiskFactory `optional:"true"`
}

// newCreator returns the [flam.IHealthCheckCreator] of the disk checks,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IHealthCheckCreator {
	return &creator{config: args.Config, disks: args.Disks}
}

// creator is the [flam.IHealthCheckCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
	disks  flam.IDiskFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the check of the configuration, failing with
// ErrInvalidField for an invalid field, ErrNoFactory without disk
// factory, and ErrUnknownDisk when the factory has no disk of the id.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IHealthCheck, error) {
	id := bag.String(factory.ConfigFieldId)

	kind, ok := health.ParseKind(settings.Field(c.config, bag, health.ConfigFieldKind, health.DefaultKindConfigPath, defaultKind))
	if !ok {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, health.ConfigFieldKind)
	}
	path, ok := bag.Get(ConfigFieldPath, defaultPath).(string)
	if !ok || path == "" {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, ConfigFieldPath)
	}
	diskId, ok := settings.Field(c.config, bag, ConfigFieldDisk, "", id).(string)
	switch {
	case !ok || diskId == "":
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, ConfigFieldDisk)
	case c.disks == nil:
		return nil, newErr(ErrNoFactory, creatorCreateMessage, id, nil)
	case !c.disks.Has(diskId):
		return nil, newErr(ErrUnknownDisk, creatorCreateMessage, id, nil).
			With(ErrorFieldDisk, diskId)
	}

	return health.NewCheck(id, kind, func(context.Context) error {
		return c.stat(id, diskId, path)
	})
}

// Messages of the errors returned by the creator and the checks.
const (
	creatorCreateMessage = "unable to create disk health check"
	checkMessage         = "disk health check failed"
)

// stat stats the path of the disk, failing with ErrDisk when the disk
// cannot be taken, and ErrStat when the path cannot be stated.
func (c *creator) stat(
	id string,
	diskId string,
	path string,
) error {
	d, err := c.disks.Get(diskId)
	if err != nil {
		return newErr(ErrDisk, checkMessage, id, err).
			With(ErrorFieldDisk, diskId)
	}
	if _, err := d.Stat(path); err != nil {
		return newErr(ErrStat, checkMessage, id, err).
			With(ErrorFieldDisk, diskId).
			With(ErrorFieldPath, path)
	}
	return nil
}
