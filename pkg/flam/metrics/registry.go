package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// registryArgs are the dependencies of the registry, taken from the
// container. The group tag repeats CollectorGroup, as a struct tag cannot
// use a constant.
type registryArgs struct {
	dig.In

	Config     flam.IConfig
	Collectors []prometheus.Collector `group:"flam.metrics.collectors"`
}

// newRegistry returns the registry holding the Go runtime and the process
// collectors, unless disabled, and the collectors of the CollectorGroup
// group, in the order the container gives them, failing on the first one
// that is nil or cannot be registered.
func newRegistry(
	args registryArgs,
) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()
	if args.Config.Bool(GoCollectorConfigPath, defaultGoCollector) {
		registry.MustRegister(collectors.NewGoCollector())
	}
	if args.Config.Bool(ProcessCollectorConfigPath, defaultProcessCollector) {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
	for _, collector := range args.Collectors {
		if value.IsNil(collector) {
			return nil, newErrCollector(ErrNilCollector, registryNewMessage, collector, nil)
		}
		if err := registry.Register(collector); err != nil {
			return nil, newErrCollector(ErrRegister, registryNewMessage, collector, err)
		}
	}
	return registry, nil
}

// registryNewMessage is the message of the errors of the registry
// creation.
const registryNewMessage = "unable to create metrics registry"
