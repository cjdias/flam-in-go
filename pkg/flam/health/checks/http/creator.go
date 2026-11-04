package http

import (
	"context"
	"fmt"
	"io"
	"math"
	nethttp "net/http"
	"net/url"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the client factory only when provided.
type creatorArgs struct {
	dig.In

	Config  flam.IConfig
	Clients flam.IClientFactory `optional:"true"`
}

// newCreator returns the [flam.IHealthCheckCreator] of the http checks,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IHealthCheckCreator {
	return &creator{config: args.Config, clients: args.Clients}
}

// creator is the [flam.IHealthCheckCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config  flam.IConfig
	clients flam.IClientFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the check of the configuration, failing with
// ErrInvalidField for an invalid field, ErrNoFactory when a client is set
// without client factory, and ErrUnknownClient when the factory has no
// client of the id.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IHealthCheck, error) {
	id := bag.String(factory.ConfigFieldId)
	invalid := func(field string) error {
		return newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, field)
	}

	kind, ok := health.ParseKind(settings.Field(c.config, bag, health.ConfigFieldKind, health.DefaultKindConfigPath, defaultKind))
	if !ok {
		return nil, invalid(health.ConfigFieldKind)
	}
	client, ok := bag.Get(ConfigFieldClient, defaultClient).(string)
	if !ok {
		return nil, invalid(ConfigFieldClient)
	}
	target, ok := bag.Get(ConfigFieldURL, "").(string)
	var template *nethttp.Request
	if ok {
		template, ok = creatorRequest(target, client != "")
	}
	if !ok {
		return nil, invalid(ConfigFieldURL)
	}
	low, ok := creatorStatus(bag.Get(ConfigFieldMinStatus, defaultMinStatus), minStatus)
	if !ok {
		return nil, invalid(ConfigFieldMinStatus)
	}
	high, ok := creatorStatus(bag.Get(ConfigFieldMaxStatus, defaultMaxStatus), low)
	if !ok {
		return nil, invalid(ConfigFieldMaxStatus)
	}
	timeout, ok := settings.Duration(bag.Get(ConfigFieldTimeout, defaultTimeout))
	if !ok {
		return nil, invalid(ConfigFieldTimeout)
	}
	switch {
	case client == "":
	case c.clients == nil:
		return nil, newErr(ErrNoFactory, creatorCreateMessage, id, nil)
	case !c.clients.Has(client):
		return nil, newErr(ErrUnknownClient, creatorCreateMessage, id, nil).
			With(ErrorFieldClient, client)
	}

	r := &request{
		clients:  c.clients,
		id:       id,
		url:      target,
		client:   client,
		template: template,
		low:      low,
		high:     high,
		timeout:  timeout,
	}
	return health.NewCheck(id, kind, r.check)
}

// Messages of the errors returned by the creator and the checks.
const (
	creatorCreateMessage = "unable to create http health check"
	checkMessage         = "http health check failed"
)

// request is the request of a check, sent with the client of the
// factory when set, or else from the template.
type request struct {
	clients  flam.IClientFactory
	id       string
	url      string
	client   string
	template *nethttp.Request
	low      int
	high     int
	timeout  time.Duration
}

// check requests the URL with GET, with the client when set, failing with
// ErrClient when the client cannot be taken, ErrRequest when the request
// fails, and ErrStatus when the status of the response is out of range.
func (r *request) check(
	ctx context.Context,
) error {
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}
	response, err := r.send(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxBodyDrain))
		_ = response.Body.Close()
	}()
	if status := response.StatusCode; status < r.low || status > r.high {
		return newErr(fmt.Errorf("%w: %d", ErrStatus, status), checkMessage, r.id, nil).
			With(ErrorFieldURL, r.url).
			With(ErrorFieldStatus, status)
	}
	return nil
}

// send sends the GET request of the URL, with the client when set.
func (r *request) send(
	ctx context.Context,
) (*nethttp.Response, error) {
	if r.client == "" {
		response, err := nethttp.DefaultClient.Do(r.template.Clone(ctx))
		if err != nil {
			return nil, newErr(ErrRequest, checkMessage, r.id, err).
				With(ErrorFieldURL, r.url)
		}
		return response, nil
	}
	client, err := r.clients.Get(r.client)
	if err != nil {
		return nil, newErr(ErrClient, checkMessage, r.id, err).
			With(ErrorFieldClient, r.client)
	}
	response, err := client.Request(ctx, nethttp.MethodGet, r.url, nil)
	if err != nil {
		return nil, newErr(ErrRequest, checkMessage, r.id, err).
			With(ErrorFieldClient, r.client).
			With(ErrorFieldURL, r.url)
	}
	return response, nil
}

// creatorRequest returns the template of the GET request of the URL,
// nil with a client, and whether the URL is valid: any URL, resolved
// against the base URL of the client, with one, or else an absolute http
// or https URL.
func creatorRequest(
	text string,
	withClient bool,
) (*nethttp.Request, bool) {
	if withClient {
		_, err := url.Parse(text)
		return nil, err == nil
	}
	req, err := nethttp.NewRequest(nethttp.MethodGet, text, nil)
	if err != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.Host == "" {
		return nil, false
	}
	return req, true
}

// creatorStatus returns the status of the value, an int, an int64 or an
// integral float64, as decoded from JSON, and whether it is one, from
// lowest to 599.
func creatorStatus(
	value any,
	lowest int,
) (int, bool) {
	status := -1
	switch v := value.(type) {
	case int:
		status = v
	case int64:
		if v >= minStatus && v <= maxStatus {
			status = int(v)
		}
	case float64:
		if v == math.Trunc(v) && v >= minStatus && v <= maxStatus {
			status = int(v)
		}
	}
	return status, status >= lowest && status <= maxStatus
}
