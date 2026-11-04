package health

import (
	"slices"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewBundle returns the [flam.IBundle] of the health feature, providing
// to the application container:
//   - a single [flam.IHealthRegistry], empty when created, waiting for
//     each check as configured at [TimeoutConfigPath], and reusing its
//     result as configured at [CacheConfigPath], reading the time from
//     the [flam.IClock] of the container;
//   - a single [flam.IHealthCheckFactory], creating the checks configured
//     at [CheckConfigPath] with the creators of the [CheckCreatorGroup]
//     group.
//
// They are created on first use, reading the configuration once. It
// provides no check driver: register them with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/health/checks].
//
// The bundle implements [flam.IBootableBundle]: when the application
// boots, it adds to the registry the checks of the [CheckGroup] group,
// then every configured check, each in id order, failing with ErrCheck,
// at the first check that cannot be created or added. The checks are
// added at boot, rather than when the registry is created, so that the
// registry can be taken by what the checks depend on, such as the REST
// endpoints of
// [github.com/cjdias/flam-in-go/pkg/flam/health/endpoints], served by a
// server of the kennel a check observes.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the check factory, and so its checks. As it is
// taken from the container, closing an application that never used it
// creates it first, and fails when it cannot be created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time checks that *bundleImpl implements IBootableBundle and
// IClosableBundle
var (
	_ flam.IBootableBundle = (*bundleImpl)(nil)
	_ flam.IClosableBundle = (*bundleImpl)(nil)
)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newRegistry).
		QueueProvider(newCheckFactory).
		Register()
}

func (b *bundleImpl) Boot(
	app flam.IApp,
) error {
	return app.Invoke(b.addChecks)
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeCheckFactory)
}

// checkFactoryArgs are the dependencies of the check factory, taken from
// the container. The group tag repeats CheckCreatorGroup, as a struct tag
// cannot use a constant.
type checkFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IHealthCheckCreator `group:"flam.health.checks.creator"`
}

// newCheckFactory returns the check factory creating the checks
// configured at CheckConfigPath with the creators of the
// CheckCreatorGroup group.
func newCheckFactory(
	args checkFactoryArgs,
) (flam.IHealthCheckFactory, error) {
	return factory.New(args.Config.Bag(CheckConfigPath), args.Creators...)
}

// addChecksArgs are the dependencies of the boot of the bundle, taken
// from the container. The group tag repeats CheckGroup.
type addChecksArgs struct {
	dig.In

	Registry flam.IHealthRegistry
	Factory  flam.IHealthCheckFactory
	Checks   []flam.IHealthCheck `group:"flam.health.checks"`
}

// addChecks adds to the registry the checks of the group, in id order, as
// the container shuffles the group, then the configured checks, failing
// with ErrCheck at the first one that cannot be created or added.
func (b *bundleImpl) addChecks(
	args addChecksArgs,
) error {
	checks := slices.SortedStableFunc(slices.Values(args.Checks), func(a, b flam.IHealthCheck) int {
		return strings.Compare(checkId(a), checkId(b))
	})
	for _, check := range checks {
		if err := args.Registry.Add(check); err != nil {
			return newErr(ErrCheck, bundleBootMessage, checkId(check), err)
		}
	}
	for _, id := range args.Factory.Available() {
		check, err := args.Factory.Get(id)
		if err == nil {
			err = args.Registry.Add(check)
		}
		if err != nil {
			return newErr(ErrCheck, bundleBootMessage, id, err)
		}
	}
	return nil
}

func (b *bundleImpl) closeCheckFactory(
	f flam.IHealthCheckFactory,
) error {
	return f.Close()
}

// bundleBootMessage is the message of the errors returned by the boot of
// the bundle.
const bundleBootMessage = "unable to boot health bundle"

// checkId returns the id of the check, or an empty id for a nil one.
func checkId(
	check flam.IHealthCheck,
) string {
	if value.IsNil(check) {
		return ""
	}
	return check.Id()
}
