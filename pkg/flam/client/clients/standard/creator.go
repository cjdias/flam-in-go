package standard

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newCreator returns the [flam.IClientCreator] of the standard clients,
// reading the defaults of the missing fields from the configuration, their
// circuit breakers from the factory, and waiting their retries with the
// clock. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	config flam.IConfig,
	clock flam.IClock,
	breakers flam.IClientCircuitBreakerFactory,
) flam.IClientCreator {
	return &creator{
		config:   config,
		clock:    clock,
		breakers: breakers,
	}
}

// creator is the [flam.IClientCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config   flam.IConfig
	clock    flam.IClock
	breakers flam.IClientCircuitBreakerFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the client of the configuration, with an HTTP transport
// of its own, and its circuit breaker, when it has one.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IClient, error) {
	id := bag.String(factory.ConfigFieldId)

	cfg, err := c.read(id, bag)
	if err != nil {
		return nil, err
	}

	var breaker flam.IClientCircuitBreaker
	if cfg.breaker != "" {
		if breaker, err = c.breakers.Get(cfg.breaker); err != nil {
			return nil, newErr(ErrBreaker, creatorCreateMessage, id, err).
				With(ErrorFieldBreaker, cfg.breaker)
		}
	}

	return &httpClient{
		id:            id,
		baseURL:       cfg.baseURL,
		headers:       cfg.headers,
		breaker:       breaker,
		retries:       cfg.retries,
		retryDelay:    cfg.retryDelay,
		retryMaxDelay: cfg.retryMaxDelay,
		clock:         c.clock,
		http: &http.Client{
			Timeout: cfg.timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   transportDialTimeout,
					KeepAlive: transportKeepAlive,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          transportMaxIdleConns,
				IdleConnTimeout:       transportIdleConnTimeout,
				TLSHandshakeTimeout:   transportTLSHandshakeTimeout,
				ExpectContinueTimeout: transportExpectContinueTimeout,
			},
		},
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard client"

// clientConfig is the configuration of a client, read by the creator.
type clientConfig struct {
	baseURL       *url.URL
	timeout       time.Duration
	headers       http.Header
	breaker       string
	retries       int
	retryDelay    time.Duration
	retryMaxDelay time.Duration
}

// read returns the configuration of the client of the id, from its fields
// or their defaults, failing with ErrInvalidField for an invalid one.
func (c *creator) read(
	id string,
	bag flam.IBag,
) (*clientConfig, error) {
	invalid := func(field string) error {
		return newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, field)
	}

	cfg := &clientConfig{}
	var ok bool
	if cfg.baseURL, ok = baseURL(c.field(bag, ConfigFieldBaseURL, "", "")); !ok {
		return nil, invalid(ConfigFieldBaseURL)
	}
	if cfg.timeout, ok = settings.Duration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout)); !ok {
		return nil, invalid(ConfigFieldTimeout)
	}
	if cfg.headers, ok = headers(c.field(bag, ConfigFieldHeaders, DefaultHeadersConfigPath, nil)); !ok {
		return nil, invalid(ConfigFieldHeaders)
	}
	if cfg.breaker, ok = c.field(bag, ConfigFieldBreaker, client.DefaultBreakerConfigPath, "").(string); !ok {
		return nil, invalid(ConfigFieldBreaker)
	}
	if cfg.retries, ok = c.field(bag, ConfigFieldRetries, DefaultRetriesConfigPath, defaultRetries).(int); !ok || cfg.retries < 0 {
		return nil, invalid(ConfigFieldRetries)
	}
	if cfg.retryDelay, ok = settings.Duration(c.field(bag, ConfigFieldRetryDelay, DefaultRetryDelayConfigPath, defaultRetryDelay)); !ok {
		return nil, invalid(ConfigFieldRetryDelay)
	}
	if cfg.retryMaxDelay, ok = settings.Duration(c.field(bag, ConfigFieldRetryMaxDelay, DefaultRetryMaxDelayConfigPath, defaultRetryMaxDelay)); !ok || cfg.retryMaxDelay < cfg.retryDelay {
		return nil, invalid(ConfigFieldRetryMaxDelay)
	}
	return cfg, nil
}

// field returns the value of the field of the client configuration, or
// else the one stored in the configuration at the default path, when not
// empty, or else the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// baseURL returns the base URL of the value, nil for an empty string, and
// whether the value is an empty string or an absolute URL with a host.
func baseURL(
	value any,
) (*url.URL, bool) {
	raw, ok := value.(string)
	switch {
	case !ok:
		return nil, false
	case raw == "":
		return nil, true
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return nil, false
	}
	return parsed, true
}

// headers returns the headers of the value, a bag or a map of header
// names and string values, such as a map[string]string, nil being no header, and whether the value is
// one.
func headers(
	value any,
) (http.Header, bool) {
	result := http.Header{}
	switch values := value.(type) {
	case nil:
		return result, true
	case map[string]string:
		for name, v := range values {
			result.Set(name, v)
		}
		return result, true
	case map[string]any:
		return headers(container.Bag(values))
	case flam.IBag:
		for name, v := range values.All() {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			result.Set(name, s)
		}
		return result, true
	}
	return nil, false
}
