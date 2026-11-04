package metrics

import (
	"errors"
	"math"
	"reflect"
	"slices"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The registerer is optional, so that registering the driver
// does not require the metrics feature: its absence only fails the
// creation of an interceptor.
type creatorArgs struct {
	dig.In

	Config     flam.IConfig
	Clock      flam.IClock
	Registerer prometheus.Registerer `optional:"true"`
}

// newCreator returns the [flam.IGrpcInterceptorCreator] of the metrics
// interceptors, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IGrpcInterceptorCreator {
	return &creator{
		config:     args.Config,
		clock:      args.Clock,
		registerer: args.Registerer,
	}
}

// creator is the [flam.IGrpcInterceptorCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config     flam.IConfig
	clock      flam.IClock
	registerer prometheus.Registerer
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the interceptor of the namespace and the buckets of the
// configuration, or of their defaults, after registering its metrics, or
// taking those registered already by an interceptor of the same
// namespace. When one of its metrics cannot be registered, those it
// registered are unregistered.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IGrpcInterceptor, error) {
	id := bag.String(factory.ConfigFieldId)

	namespace, ok := c.field(bag, ConfigFieldNamespace, DefaultNamespaceConfigPath, defaultNamespace).(string)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldNamespace, nil)
	}
	buckets, ok := creatorBuckets(c.field(bag, ConfigFieldBuckets, DefaultBucketsConfigPath, defaultBuckets()))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldBuckets, nil)
	}
	if c.registerer == nil {
		return nil, newErrCreate(ErrMissingRegisterer, creatorCreateMessage, id, "", nil)
	}

	labels := []string{LabelService, LabelMethod, LabelCode}
	r := &creatorRegistration{registerer: c.registerer}
	handled := creatorRegister(r, prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: Subsystem,
		Name:      MetricHandled,
		Help:      creatorHandledHelp,
	}, labels))
	duration := creatorRegister(r, prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: Subsystem,
		Name:      MetricDuration,
		Help:      creatorDurationHelp,
		Buckets:   buckets,
	}, labels))
	if r.err != nil {
		r.rollback()
		return nil, newErrCreate(ErrRegister, creatorCreateMessage, id, "", r.err)
	}

	return &interceptor{
		id:       id,
		clock:    c.clock,
		handled:  handled,
		duration: duration,
	}, nil
}

// field returns the value of the field of the interceptor configuration,
// or else the one stored in the configuration at the default path, or
// else the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorRegistration registers the metrics of an interceptor, keeping the
// first error, after which it registers nothing, and the metrics it
// registered, to unregister them on failure.
type creatorRegistration struct {
	registerer prometheus.Registerer
	registered []prometheus.Collector
	err        error
}

// rollback unregisters the metrics registered.
func (r *creatorRegistration) rollback() {
	for _, collector := range r.registered {
		r.registerer.Unregister(collector)
	}
}

// creatorRegister registers the metric, returning it, or the metric of the
// same type registered already under the same description, or the zero
// value once the registration failed.
func creatorRegister[C prometheus.Collector](
	r *creatorRegistration,
	collector C,
) C {
	var zero C
	if r.err != nil {
		return zero
	}
	err := r.registerer.Register(collector)
	if err == nil {
		r.registered = append(r.registered, collector)
		return collector
	}
	var registered prometheus.AlreadyRegisteredError
	if errors.As(err, &registered) {
		if existing, ok := registered.ExistingCollector.(C); ok {
			return existing
		}
	}
	r.err = err
	return zero
}

// creatorBuckets returns the buckets of the value, a []float64 or a []any
// of whole or floating numbers, and whether the value is a non-empty list
// of increasing numbers, none of which is NaN.
func creatorBuckets(
	value any,
) ([]float64, bool) {
	var buckets []float64
	switch list := value.(type) {
	case []float64:
		buckets = slices.Clone(list)
	case []any:
		buckets = make([]float64, len(list))
		for i, item := range list {
			bucket, ok := creatorNumber(item)
			if !ok {
				return nil, false
			}
			buckets[i] = bucket
		}
	}
	if len(buckets) == 0 {
		return nil, false
	}
	for i, bucket := range buckets {
		if math.IsNaN(bucket) || (i > 0 && bucket <= buckets[i-1]) {
			return nil, false
		}
	}
	return buckets, true
}

// creatorNumber returns the value of the number, of any integer or float
// kind, as a float64, and whether the value is one.
func creatorNumber(
	value any,
) (float64, bool) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	}
	return 0, false
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create metrics grpc interceptor"

// Help texts of the metrics.
const (
	creatorHandledHelp  = "Number of gRPC calls served, by service, method and code."
	creatorDurationHelp = "Time taken to serve the gRPC calls, in seconds, by service, method and code."
)
