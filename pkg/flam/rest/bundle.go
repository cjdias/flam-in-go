package rest

import (
	"errors"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// NewBundle returns the [flam.IBundle] of the REST feature, providing to
// the application container:
//   - a single [flam.IRestServerFactory], creating the servers configured
//     at [ServerConfigPath] with the creators of the [ServerCreatorGroup]
//     group;
//   - a single [flam.IRestLoggerAdaptorFactory], creating the logger
//     adaptors configured at [LoggerConfigPath] with the creators of the
//     [LoggerCreatorGroup] group;
//   - a single [flam.IRestMiddlewareFactory], creating the middlewares
//     configured at [MiddlewareConfigPath] with the creators of the
//     [MiddlewareCreatorGroup] group;
//   - a single *[schema.Decoder], returned by [NewDecoder];
//   - every configured server, in the [watchdog.ProcessGroup] group, so
//     that the kennel of
//     [github.com/cjdias/flam-in-go/pkg/flam/watchdog] runs them, as
//     configured at [watchdog.ProcessConfigPath] by their id.
//
// The factories are created on first use, reading the configuration once.
// It provides no driver: register them with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/servers],
// [github.com/cjdias/flam-in-go/pkg/flam/rest/loggers] and
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares], and register
// the watchdog bundle to run the servers.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the server factory, and so its servers, then the
// middleware and the logger adaptor factories, even when one fails,
// returning their errors joined.
//
// [schema.Decoder]: https://pkg.go.dev/github.com/gorilla/schema#Decoder
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IClosableBundle
var _ flam.IClosableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newLoggerFactory).
		QueueProvider(newMiddlewareFactory).
		QueueProvider(newServerFactory).
		QueueProvider(NewDecoder).
		QueueProvider(b.provideProcesses, dig.Group(watchdog.ProcessGroup+",flatten")).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeServerFactory),
		app.Invoke(b.closeMiddlewareFactory),
		app.Invoke(b.closeLoggerFactory),
	)
}

// provideProcesses provides the configured servers, in id order, as
// processes of the kennel, failing with the error of the first one that
// cannot be created.
func (b *bundleImpl) provideProcesses(
	servers flam.IRestServerFactory,
) ([]flam.IWatchdogProcess, error) {
	ids := servers.Available()
	processes := make([]flam.IWatchdogProcess, 0, len(ids))
	for _, id := range ids {
		server, err := servers.Get(id)
		if err != nil {
			return nil, newErrJoined(ErrServer, bundleProcessesMessage, err).
				With(ErrorFieldID, id)
		}
		processes = append(processes, server)
	}
	return processes, nil
}

func (b *bundleImpl) closeServerFactory(
	f flam.IRestServerFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeMiddlewareFactory(
	f flam.IRestMiddlewareFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeLoggerFactory(
	f flam.IRestLoggerAdaptorFactory,
) error {
	return f.Close()
}

// bundleProcessesMessage is the message of the errors of the servers
// provided as processes.
const bundleProcessesMessage = "unable to provide rest server process"
