package kennel

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the kennel only when provided.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Kennel flam.IKennel `optional:"true"`
}

// newCreator returns the [flam.IHealthCheckCreator] of the kennel checks,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IHealthCheckCreator {
	return &creator{config: args.Config, kennel: args.Kennel}
}

// creator is the [flam.IHealthCheckCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
	kennel flam.IKennel
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the check of the configuration, failing with
// ErrInvalidField for an invalid field, ErrNoKennel without kennel, and
// ErrUnknownProcess for a process the kennel does not hold.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IHealthCheck, error) {
	id := bag.String(factory.ConfigFieldId)

	kind, ok := health.ParseKind(settings.Field(c.config, bag, health.ConfigFieldKind, health.DefaultKindConfigPath, defaultKind))
	if !ok {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id).
			With(ErrorFieldField, health.ConfigFieldKind)
	}
	ids, ok := settings.Strings(bag.Get(ConfigFieldProcesses))
	if !ok {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id).
			With(ErrorFieldField, ConfigFieldProcesses)
	}
	if c.kennel == nil {
		return nil, newErr(ErrNoKennel, creatorCreateMessage, id)
	}
	if !bag.Has(ConfigFieldProcesses) {
		ids = c.kennel.Available()
	}
	for _, processId := range ids {
		if !c.kennel.Has(processId) {
			return nil, newErr(ErrUnknownProcess, creatorCreateMessage, id).
				With(ErrorFieldProcess, processId)
		}
	}
	ids = slices.Sorted(slices.Values(ids))

	return health.NewCheck(id, kind, func(context.Context) error {
		return c.check(id, ids)
	})
}

// Messages of the errors returned by the creator and the checks.
const (
	creatorCreateMessage = "unable to create kennel health check"
	checkMessage         = "kennel health check failed"
)

// check fails with ErrNotRunning, while the kennel runs its processes,
// when processes of the ids, active in the kennel, are not running,
// giving their ids.
func (c *creator) check(
	id string,
	ids []string,
) error {
	if !c.kennel.IsRunning() {
		return nil
	}
	var stopped []string
	for _, processId := range ids {
		process, ok := c.kennel.Process(processId)
		if ok && c.kennel.IsActive(processId) && !process.IsRunning() {
			stopped = append(stopped, processId)
		}
	}
	if len(stopped) == 0 {
		return nil
	}
	return newErr(fmt.Errorf("%w: %s", ErrNotRunning, strings.Join(stopped, ", ")), checkMessage, id).
		With(ErrorFieldProcesses, stopped)
}
