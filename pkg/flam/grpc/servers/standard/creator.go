package standard

import (
	"crypto/tls"
	"time"

	"go.uber.org/dig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The health registry is optional, so that registering the
// driver does not require the health feature: the health service then
// tells the server serving as long as it runs. The group tag repeats
// flamgrpc.ServiceGroup, as a struct tag cannot use a constant.
type creatorArgs struct {
	dig.In

	Config       flam.IConfig
	Clock        flam.IClock
	Loggers      flam.IGrpcLoggerAdaptorFactory
	Interceptors flam.IGrpcInterceptorFactory
	Health       flam.IHealthRegistry        `optional:"true"`
	Services     []flam.IGrpcServiceRegister `group:"flam.grpc.services"`
}

// newCreator returns the [flam.IGrpcServerCreator] of the standard
// servers, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IGrpcServerCreator {
	return &creator{
		config:       args.Config,
		clock:        args.Clock,
		loggers:      args.Loggers,
		interceptors: args.Interceptors,
		health:       args.Health,
		services:     args.Services,
	}
}

// creator is the [flam.IGrpcServerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config       flam.IConfig
	clock        flam.IClock
	loggers      flam.IGrpcLoggerAdaptorFactory
	interceptors flam.IGrpcInterceptorFactory
	health       flam.IHealthRegistry
	services     []flam.IGrpcServiceRegister
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the server of the configuration, serving the services of
// the registers of its group, along with the health and the reflection
// services when enabled, through its interceptors.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IGrpcServer, error) {
	id := bag.String(factory.ConfigFieldId)

	cfg, err := c.read(id, bag)
	if err != nil {
		return nil, err
	}

	var logger flam.IGrpcLoggerAdaptor
	if cfg.logger != "" {
		if logger, err = c.loggers.Get(cfg.logger); err != nil {
			return nil, newErr(ErrLogger, creatorCreateMessage, id, err).
				With(ErrorFieldLogger, cfg.logger)
		}
	}

	var unaries []grpc.UnaryServerInterceptor
	var streams []grpc.StreamServerInterceptor
	for _, interceptorId := range cfg.interceptors {
		interceptor, err := c.interceptors.Get(interceptorId)
		if err != nil {
			return nil, newErr(ErrInterceptor, creatorCreateMessage, id, err).
				With(ErrorFieldInterceptor, interceptorId)
		}
		if unary := interceptor.Unary(); unary != nil {
			unaries = append(unaries, unary)
		}
		if stream := interceptor.Stream(); stream != nil {
			streams = append(streams, stream)
		}
	}

	options := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaries...),
		grpc.ChainStreamInterceptor(streams...),
		grpc.ConnectionTimeout(cfg.connectionTimeout),
		grpc.MaxRecvMsgSize(cfg.maxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.maxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     cfg.maxConnectionIdle,
			MaxConnectionAge:      cfg.maxConnectionAge,
			MaxConnectionAgeGrace: cfg.maxConnectionAgeGrace,
			Time:                  cfg.keepaliveTime,
			Timeout:               cfg.keepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             cfg.keepaliveMinTime,
			PermitWithoutStream: cfg.keepalivePermitWithoutStream,
		}),
	}
	if cfg.certFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.certFile, cfg.keyFile)
		if err != nil {
			return nil, newErr(ErrCertificate, creatorCreateMessage, id, err)
		}
		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		})))
	}

	s := grpc.NewServer(options...)
	r := &registrar{server: s}
	for _, register := range c.services {
		if register.Group() != cfg.group {
			continue
		}
		if err := register.Register(r); err != nil {
			return nil, newErr(ErrRegister, creatorCreateMessage, id, err)
		}
	}
	var h *health
	if cfg.health {
		h = newHealth(c.health, c.clock, cfg.healthInterval)
		grpc_health_v1.RegisterHealthServer(r, h)
	}
	if cfg.reflection {
		reflection.Register(r)
	}
	if r.invalid {
		return nil, newErr(ErrRegister, creatorCreateMessage, id, ErrInvalidService).
			With(ErrorFieldService, r.service)
	}
	if h != nil {
		h.init(s.GetServiceInfo())
	}

	return &server{
		id:              id,
		address:         cfg.address,
		logger:          logger,
		grpc:            s,
		health:          h,
		shutdownTimeout: cfg.shutdownTimeout,
		stopped:         make(chan struct{}),
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard grpc server"

// serverConfig is the configuration of a server, read by the creator.
type serverConfig struct {
	address                      string
	group                        string
	logger                       string
	interceptors                 []string
	certFile                     string
	keyFile                      string
	connectionTimeout            time.Duration
	maxRecvMsgSize               int
	maxSendMsgSize               int
	keepaliveTime                time.Duration
	keepaliveTimeout             time.Duration
	keepaliveMinTime             time.Duration
	keepalivePermitWithoutStream bool
	maxConnectionIdle            time.Duration
	maxConnectionAge             time.Duration
	maxConnectionAgeGrace        time.Duration
	reflection                   bool
	health                       bool
	healthInterval               time.Duration
	shutdownTimeout              time.Duration
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
	if cfg.address, ok = c.field(bag, ConfigFieldAddress, DefaultAddressConfigPath, defaultAddress).(string); !ok || cfg.address == "" {
		return nil, invalid(ConfigFieldAddress)
	}
	if cfg.group, ok = c.field(bag, ConfigFieldGroup, "", id).(string); !ok || cfg.group == "" {
		return nil, invalid(ConfigFieldGroup)
	}
	if cfg.logger, ok = c.field(bag, ConfigFieldLogger, flamgrpc.DefaultLoggerConfigPath, "").(string); !ok {
		return nil, invalid(ConfigFieldLogger)
	}
	if cfg.interceptors, ok = settings.Strings(c.field(bag, ConfigFieldInterceptors, flamgrpc.DefaultInterceptorsConfigPath, nil)); !ok {
		return nil, invalid(ConfigFieldInterceptors)
	}
	if cfg.certFile, ok = c.field(bag, ConfigFieldCertFile, "", "").(string); !ok {
		return nil, invalid(ConfigFieldCertFile)
	}
	if cfg.keyFile, ok = c.field(bag, ConfigFieldKeyFile, "", "").(string); !ok || (cfg.keyFile == "") != (cfg.certFile == "") {
		return nil, invalid(ConfigFieldKeyFile)
	}
	if cfg.connectionTimeout, ok = settings.Duration(c.field(bag, ConfigFieldConnectionTimeout, DefaultConnectionTimeoutConfigPath, defaultConnectionTimeout)); !ok || cfg.connectionTimeout == 0 {
		return nil, invalid(ConfigFieldConnectionTimeout)
	}
	if cfg.maxRecvMsgSize, ok = c.field(bag, ConfigFieldMaxRecvMsgSize, DefaultMaxRecvMsgSizeConfigPath, defaultMaxRecvMsgSize).(int); !ok || cfg.maxRecvMsgSize <= 0 {
		return nil, invalid(ConfigFieldMaxRecvMsgSize)
	}
	if cfg.maxSendMsgSize, ok = c.field(bag, ConfigFieldMaxSendMsgSize, DefaultMaxSendMsgSizeConfigPath, defaultMaxSendMsgSize).(int); !ok || cfg.maxSendMsgSize <= 0 {
		return nil, invalid(ConfigFieldMaxSendMsgSize)
	}
	for _, d := range []struct {
		value       *time.Duration
		field       string
		defaultPath string
		builtIn     time.Duration
	}{
		{&cfg.keepaliveTime, ConfigFieldKeepaliveTime, DefaultKeepaliveTimeConfigPath, defaultKeepaliveTime},
		{&cfg.keepaliveTimeout, ConfigFieldKeepaliveTimeout, DefaultKeepaliveTimeoutConfigPath, defaultKeepaliveTimeout},
		{&cfg.keepaliveMinTime, ConfigFieldKeepaliveMinTime, DefaultKeepaliveMinTimeConfigPath, defaultKeepaliveMinTime},
		{&cfg.maxConnectionIdle, ConfigFieldMaxConnectionIdle, DefaultMaxConnectionIdleConfigPath, defaultMaxConnectionIdle},
		{&cfg.maxConnectionAge, ConfigFieldMaxConnectionAge, DefaultMaxConnectionAgeConfigPath, defaultMaxConnectionAge},
		{&cfg.maxConnectionAgeGrace, ConfigFieldMaxConnectionAgeGrace, DefaultMaxConnectionAgeGraceConfigPath, defaultMaxConnectionAgeGrace},
		{&cfg.shutdownTimeout, ConfigFieldShutdownTimeout, DefaultShutdownTimeoutConfigPath, defaultShutdownTimeout},
	} {
		if *d.value, ok = settings.Duration(c.field(bag, d.field, d.defaultPath, d.builtIn)); !ok {
			return nil, invalid(d.field)
		}
	}
	if cfg.keepalivePermitWithoutStream, ok = c.field(bag, ConfigFieldKeepalivePermitWithoutStream, DefaultKeepalivePermitWithoutStreamConfigPath, defaultKeepalivePermitWithoutStream).(bool); !ok {
		return nil, invalid(ConfigFieldKeepalivePermitWithoutStream)
	}
	if cfg.reflection, ok = c.field(bag, ConfigFieldReflection, DefaultReflectionConfigPath, defaultReflection).(bool); !ok {
		return nil, invalid(ConfigFieldReflection)
	}
	if cfg.health, ok = c.field(bag, ConfigFieldHealth, DefaultHealthConfigPath, defaultHealth).(bool); !ok {
		return nil, invalid(ConfigFieldHealth)
	}
	if cfg.healthInterval, ok = settings.Duration(c.field(bag, ConfigFieldHealthInterval, DefaultHealthIntervalConfigPath, defaultHealthInterval)); !ok || cfg.healthInterval == 0 {
		return nil, invalid(ConfigFieldHealthInterval)
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
