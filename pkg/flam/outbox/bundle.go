package outbox

import (
	"errors"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// NewBundle returns the [flam.IBundle] of the outbox feature, providing to
// the application container the relay of every relay configuration at
// [RelayConfigPath], in the [watchdog.ProcessGroup] group, so that the
// kennel of [github.com/cjdias/flam-in-go/pkg/flam/watchdog] runs them, as
// configured at [watchdog.ProcessConfigPath] by their process id. A relay
// publishes the messages of the [Table] table of its database connection
// with its broker, reading the time from the [flam.IClock] of the
// container. Register the bundles of the database and broker features, of
// the clock and of the watchdog along with this one.
//
// The relays are created, reading the configuration once, when the kennel
// is, failing its creation with the error of the first relay, in id
// order, that cannot be created. The kennel owns the relays and closes
// them, while the connection and broker factories keep owning the
// connections and brokers of the relays.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(b.provideProcesses, dig.Group(watchdog.ProcessGroup+",flatten")).
		Register()
}

// relayArgs are the dependencies of the relays, taken from the container,
// the connection and broker factories being optional so that the
// creation of a relay fails with an error telling which bundle is
// missing, and the logger optional so that the relays log nothing without
// one.
type relayArgs struct {
	dig.In

	Config      flam.IConfig
	Clock       flam.IClock
	Connections flam.IDatabaseConnectionFactory `optional:"true"`
	Brokers     flam.IBrokerFactory             `optional:"true"`
	Logger      flam.ILogger                    `optional:"true"`
}

// provideProcesses provides the configured relays, in id order, failing
// with the error of the first one that cannot be created.
func (b *bundleImpl) provideProcesses(
	args relayArgs,
) ([]flam.IWatchdogProcess, error) {
	configs := args.Config.Bag(RelayConfigPath, container.Bag{})
	var processes []flam.IWatchdogProcess
	for id := range configs.All() {
		bag := configs.Bag(id)
		if bag == nil {
			return nil, newErrRelay(ErrInvalidField, bundleRelayMessage, id, nil).
				With(ErrorFieldField, RelayConfigPath+"."+id)
		}
		r, err := b.newRelay(args, id, bag)
		if err != nil {
			return nil, err
		}
		processes = append(processes, r)
	}
	return processes, nil
}

// newRelay returns the relay of the id and its configuration, failing at
// the first invalid field, after creating or updating the table of its
// connection unless its migrate field is false.
func (b *bundleImpl) newRelay(
	args relayArgs,
	id string,
	bag flam.IBag,
) (*relay, error) {
	rd := &reader{config: args.Config, bag: bag}
	connection := rd.string(ConfigFieldConnection, DefaultConnectionConfigPath, "", true)
	brokerId := rd.string(ConfigFieldBroker, DefaultBrokerConfigPath, "", true)
	r := &relay{
		id:         rd.string(ConfigFieldProcess, "", processPrefix+id, true),
		relayId:    id,
		clock:      args.Clock,
		logger:     args.Logger,
		poll:       rd.duration(ConfigFieldPoll, DefaultPollConfigPath, defaultPoll, true),
		batch:      rd.int(ConfigFieldBatch, DefaultBatchConfigPath, defaultBatch, 1),
		lease:      rd.duration(ConfigFieldLease, DefaultLeaseConfigPath, defaultLease, true),
		backoff:    rd.duration(ConfigFieldBackoff, DefaultBackoffConfigPath, defaultBackoff, false),
		maxBackoff: rd.duration(ConfigFieldMaxBackoff, DefaultMaxBackoffConfigPath, defaultMaxBackoff, false),
		delete:     rd.bool(ConfigFieldDelete, DefaultDeleteConfigPath, defaultDelete),
		retention:  rd.duration(ConfigFieldRetention, DefaultRetentionConfigPath, defaultRetention, false),
		cleanup:    rd.duration(ConfigFieldCleanup, DefaultCleanupConfigPath, defaultCleanup, true),
		stop:       make(chan struct{}),
	}
	rd.check(ConfigFieldMaxBackoff, r.maxBackoff >= r.backoff)
	migrate := rd.bool(ConfigFieldMigrate, DefaultMigrateConfigPath, defaultMigrate)
	if rd.invalid != "" {
		return nil, newErrRelay(ErrInvalidField, bundleRelayMessage, id, nil).
			With(ErrorFieldField, rd.invalid)
	}

	var err error
	if args.Connections == nil {
		err = errNoConnectionFactory
	} else {
		r.conn, err = args.Connections.Get(connection)
	}
	if err != nil {
		return nil, newErrRelay(ErrConnection, bundleRelayMessage, id, err).
			With(ErrorFieldConnection, connection)
	}
	if args.Brokers == nil {
		err = errNoBrokerFactory
	} else {
		r.broker, err = args.Brokers.Get(brokerId)
	}
	if err != nil {
		return nil, newErrRelay(ErrBroker, bundleRelayMessage, id, err).
			With(ErrorFieldBroker, brokerId)
	}
	if migrate {
		if err := r.conn.AutoMigrate(&Row{}); err != nil {
			return nil, newErrRelay(ErrMigrate, bundleRelayMessage, id, err).
				With(ErrorFieldConnection, connection)
		}
	}
	return r, nil
}

// bundleRelayMessage is the message of the errors of the creation of the
// relays.
const bundleRelayMessage = "unable to create outbox relay"

// Causes of the ErrConnection and ErrBroker errors when the container
// provides no factory.
var (
	errNoConnectionFactory = errors.New("no database connection factory, register the database bundle")
	errNoBrokerFactory     = errors.New("no broker factory, register the broker bundle")
)
