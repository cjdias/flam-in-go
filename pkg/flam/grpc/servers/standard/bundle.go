package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
)

// NewBundle returns the [flam.IBundle] of the standard server driver,
// providing its [flam.IGrpcServerCreator] in the
// [flamgrpc.ServerCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the defaults of the server fields.
//
// The creator takes the [flam.IClock] from the container, and the
// [flam.IHealthRegistry] when provided: register the clock bundle, and
// the health bundle for the health service to tell the readiness of the
// application.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IConfigurableBundle
var _ flam.IConfigurableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Config(
	bag flam.IBag,
) error {
	for path, value := range map[string]any{
		DefaultAddressConfigPath:                      defaultAddress,
		DefaultConnectionTimeoutConfigPath:            defaultConnectionTimeout.String(),
		DefaultMaxRecvMsgSizeConfigPath:               defaultMaxRecvMsgSize,
		DefaultMaxSendMsgSizeConfigPath:               defaultMaxSendMsgSize,
		DefaultKeepaliveTimeConfigPath:                defaultKeepaliveTime.String(),
		DefaultKeepaliveTimeoutConfigPath:             defaultKeepaliveTimeout.String(),
		DefaultKeepaliveMinTimeConfigPath:             defaultKeepaliveMinTime.String(),
		DefaultKeepalivePermitWithoutStreamConfigPath: defaultKeepalivePermitWithoutStream,
		DefaultMaxConnectionIdleConfigPath:            defaultMaxConnectionIdle.String(),
		DefaultMaxConnectionAgeConfigPath:             defaultMaxConnectionAge.String(),
		DefaultMaxConnectionAgeGraceConfigPath:        defaultMaxConnectionAgeGrace.String(),
		DefaultReflectionConfigPath:                   defaultReflection,
		DefaultHealthConfigPath:                       defaultHealth,
		DefaultHealthIntervalConfigPath:               defaultHealthInterval.String(),
		DefaultShutdownTimeoutConfigPath:              defaultShutdownTimeout.String(),
	} {
		if err := bag.Set(path, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(flamgrpc.ServerCreatorGroup)).
		Register()
}
