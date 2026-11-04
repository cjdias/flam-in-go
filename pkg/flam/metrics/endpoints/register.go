package endpoints

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// registerArgs are the dependencies of the register, taken from the
// container. The gatherer is optional, so that a missing metrics feature
// fails the registration of the endpoint, on the servers of its group
// only, rather than the creation of every server.
type registerArgs struct {
	dig.In

	Config   flam.IConfig
	Gatherer prometheus.Gatherer `optional:"true"`
}

// newRegister returns the [flam.IRestEndpointRegister] of the metrics
// endpoint, of the group, the path and the id of the configuration, or of
// their defaults. It is the provider registered by the bundle returned
// from [NewBundle].
func newRegister(
	args registerArgs,
) flam.IRestEndpointRegister {
	return &register{
		gatherer: args.Gatherer,
		group:    args.Config.String(GroupConfigPath, defaultGroup),
		path:     args.Config.String(PathConfigPath, defaultPath),
		id:       args.Config.Int(IdConfigPath, defaultId),
	}
}

// register is the [flam.IRestEndpointRegister] of the metrics endpoint.
type register struct {
	gatherer prometheus.Gatherer
	group    string
	path     string
	id       int
}

// compile time check that *register implements IRestEndpointRegister
var _ flam.IRestEndpointRegister = (*register)(nil)

func (r *register) Group() string {
	return r.group
}

// Register registers the endpoint serving, to the GET requests of its
// path, the metrics of the gatherer in the format the scrape accepts,
// answering with a 500 status when they cannot be gathered. It fails with
// ErrMissingGatherer when the container provides no gatherer, and with
// the error of the router when the path is not a valid one.
func (r *register) Register(
	router flam.IRestRouter,
) error {
	if r.gatherer == nil {
		return newErrRegister(ErrMissingGatherer, registerRegisterMessage)
	}
	return router.Handle(flam.RestEndpoint{
		Id:      r.id,
		Pattern: http.MethodGet + " " + r.path,
	}, promhttp.HandlerFor(r.gatherer, promhttp.HandlerOpts{}))
}

// registerRegisterMessage is the message of the errors returned by
// Register.
const registerRegisterMessage = "unable to register metrics endpoint"
