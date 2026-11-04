package redis

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// collectorsArgs are the dependencies of the collectors, taken from the
// container. The factory is optional, so that, with no connection listed,
// registering the bundle does not require the Redis feature.
type collectorsArgs struct {
	dig.In

	Config      flam.IConfig
	Connections flam.IRedisConnectionFactory `optional:"true"`
}

// newCollectors returns the collectors of the statistics of the pools of
// the connections listed at ConnectionsConfigPath, or else of every
// connection configured in the factory, none without factory, failing on
// the first connection that cannot be taken. It is the provider
// registered by the bundle returned from [NewBundle].
func newCollectors(
	args collectorsArgs,
) ([]prometheus.Collector, error) {
	ids, ok := settings.Strings(args.Config.Get(ConnectionsConfigPath))
	switch {
	case !ok:
		return nil, newErr(ErrInvalidConnections, collectorsNewMessage, nil).
			With(ErrorFieldPath, ConnectionsConfigPath)
	case args.Connections == nil && len(ids) != 0:
		return nil, newErr(ErrMissingFactory, collectorsNewMessage, nil)
	case args.Connections == nil:
		return nil, nil
	case !args.Config.Has(ConnectionsConfigPath):
		ids = args.Connections.Available()
	}

	stats := make([]prometheus.Collector, 0, len(ids))
	for _, id := range ids {
		connection, err := args.Connections.Get(id)
		if err != nil {
			return nil, newErr(ErrConnection, collectorsNewMessage, err).With(ErrorFieldID, id)
		}
		stats = append(stats, newCollector(id, connection))
	}
	return stats, nil
}

// collectorsNewMessage is the message of the errors of the collectors.
const collectorsNewMessage = "unable to create metrics redis collectors"
