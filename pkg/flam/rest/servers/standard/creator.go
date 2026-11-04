package standard

import (
	"crypto/tls"
	"net/http"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The group tag repeats rest.EndpointGroup, as a struct tag
// cannot use a constant.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Loggers     flam.IRestLoggerAdaptorFactory
	Middlewares flam.IRestMiddlewareFactory
	Endpoints   []flam.IRestEndpointRegister `group:"flam.rest.endpoints"`
}

// newCreator returns the [flam.IRestServerCreator] of the standard
// servers, reading the defaults of the missing fields and the service id
// from the configuration. It is the provider registered by the bundle
// returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestServerCreator {
	return &creator{
		config:      args.Config,
		loggers:     args.Loggers,
		middlewares: args.Middlewares,
		endpoints:   args.Endpoints,
	}
}

// creator is the [flam.IRestServerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config      flam.IConfig
	loggers     flam.IRestLoggerAdaptorFactory
	middlewares flam.IRestMiddlewareFactory
	endpoints   []flam.IRestEndpointRegister
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the server of the configuration, its router holding the
// endpoints of the registers of its group, wrapped in its middlewares.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestServer, error) {
	id := bag.String(factory.ConfigFieldId)

	cfg, err := c.read(id, bag)
	if err != nil {
		return nil, err
	}

	var logger flam.IRestLoggerAdaptor
	if cfg.logger != "" {
		if logger, err = c.loggers.Get(cfg.logger); err != nil {
			return nil, newErr(ErrLogger, creatorCreateMessage, id, err).
				With(ErrorFieldLogger, cfg.logger)
		}
	}

	middlewares := make([]flam.IRestMiddleware, 0, len(cfg.middlewares))
	for _, middlewareId := range cfg.middlewares {
		middleware, err := c.middlewares.Get(middlewareId)
		if err != nil {
			return nil, newErr(ErrMiddleware, creatorCreateMessage, id, err).
				With(ErrorFieldMiddleware, middlewareId)
		}
		middlewares = append(middlewares, middleware)
	}

	var tlsConfig *tls.Config
	if cfg.certFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.certFile, cfg.keyFile)
		if err != nil {
			return nil, newErr(ErrCertificate, creatorCreateMessage, id, err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		}
	}

	router, err := rest.NewRouter(cfg.serviceId, middlewares...)
	if err != nil {
		return nil, newErr(ErrMiddleware, creatorCreateMessage, id, err)
	}
	for _, register := range c.endpoints {
		if register.Group() != cfg.group {
			continue
		}
		if err := register.Register(router); err != nil {
			return nil, newErr(ErrRegister, creatorCreateMessage, id, err)
		}
	}

	return &server{
		id:      id,
		address: cfg.address,
		logger:  logger,
		http: &http.Server{
			Handler:           router,
			ReadHeaderTimeout: cfg.readHeaderTimeout,
			ReadTimeout:       cfg.readTimeout,
			WriteTimeout:      cfg.writeTimeout,
			IdleTimeout:       cfg.idleTimeout,
			MaxHeaderBytes:    cfg.maxHeaderBytes,
			TLSConfig:         tlsConfig,
			ErrorLog:          errorLog(id, logger),
		},
		shutdownTimeout: cfg.shutdownTimeout,
		stopped:         make(chan struct{}),
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard rest server"

// serverConfig is the configuration of a server, read by the creator.
type serverConfig struct {
	serviceId         int
	address           string
	group             string
	logger            string
	middlewares       []string
	certFile          string
	keyFile           string
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	maxHeaderBytes    int
	shutdownTimeout   time.Duration
}

// read returns the configuration of the server of the id, from its fields
// or their defaults, failing with ErrInvalidField for an invalid one.
func (c *creator) read(
	id string,
	bag flam.IBag,
) (*serverConfig, error) {
	invalid := func(field string) error {
		return newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, field)
	}

	cfg := &serverConfig{}
	var ok bool
	if cfg.serviceId, ok = c.config.Get(rest.ServiceIdConfigPath, 0).(int); !ok {
		return nil, invalid(rest.ServiceIdConfigPath)
	}
	if cfg.address, ok = c.field(bag, ConfigFieldAddress, DefaultAddressConfigPath, defaultAddress).(string); !ok || cfg.address == "" {
		return nil, invalid(ConfigFieldAddress)
	}
	if cfg.group, ok = c.field(bag, ConfigFieldGroup, "", id).(string); !ok || cfg.group == "" {
		return nil, invalid(ConfigFieldGroup)
	}
	if cfg.logger, ok = c.field(bag, ConfigFieldLogger, rest.DefaultLoggerConfigPath, "").(string); !ok {
		return nil, invalid(ConfigFieldLogger)
	}
	if cfg.middlewares, ok = settings.Strings(c.field(bag, ConfigFieldMiddlewares, rest.DefaultMiddlewaresConfigPath, nil)); !ok {
		return nil, invalid(ConfigFieldMiddlewares)
	}
	if cfg.certFile, ok = c.field(bag, ConfigFieldCertFile, "", "").(string); !ok {
		return nil, invalid(ConfigFieldCertFile)
	}
	if cfg.keyFile, ok = c.field(bag, ConfigFieldKeyFile, "", "").(string); !ok || (cfg.keyFile == "") != (cfg.certFile == "") {
		return nil, invalid(ConfigFieldKeyFile)
	}
	if cfg.readHeaderTimeout, ok = settings.Duration(c.field(bag, ConfigFieldReadHeaderTimeout, DefaultReadHeaderTimeoutConfigPath, defaultReadHeaderTimeout)); !ok {
		return nil, invalid(ConfigFieldReadHeaderTimeout)
	}
	if cfg.readTimeout, ok = settings.Duration(c.field(bag, ConfigFieldReadTimeout, DefaultReadTimeoutConfigPath, defaultReadTimeout)); !ok {
		return nil, invalid(ConfigFieldReadTimeout)
	}
	if cfg.writeTimeout, ok = settings.Duration(c.field(bag, ConfigFieldWriteTimeout, DefaultWriteTimeoutConfigPath, defaultWriteTimeout)); !ok {
		return nil, invalid(ConfigFieldWriteTimeout)
	}
	if cfg.idleTimeout, ok = settings.Duration(c.field(bag, ConfigFieldIdleTimeout, DefaultIdleTimeoutConfigPath, defaultIdleTimeout)); !ok {
		return nil, invalid(ConfigFieldIdleTimeout)
	}
	if cfg.maxHeaderBytes, ok = c.field(bag, ConfigFieldMaxHeaderBytes, DefaultMaxHeaderBytesConfigPath, defaultMaxHeaderBytes).(int); !ok || cfg.maxHeaderBytes <= 0 {
		return nil, invalid(ConfigFieldMaxHeaderBytes)
	}
	if cfg.shutdownTimeout, ok = settings.Duration(c.field(bag, ConfigFieldShutdownTimeout, DefaultShutdownTimeoutConfigPath, defaultShutdownTimeout)); !ok {
		return nil, invalid(ConfigFieldShutdownTimeout)
	}
	return cfg, nil
}

// field returns the value of the field of the server configuration, or
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
