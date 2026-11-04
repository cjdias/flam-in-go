package database

import (
	"maps"
	"math"
	"reflect"
	"time"

	"go.uber.org/dig"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/backoff"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// connectionCreatorArgs are the dependencies of the connection creator,
// taken from the container. The clock is optional, so that the
// connections that are not retried can be created without the clock
// bundle.
type connectionCreatorArgs struct {
	dig.In

	Config   flam.IConfig
	Dialects flam.IDatabaseDialectFactory
	Configs  flam.IDatabaseConfigFactory
	Clock    flam.IClock `optional:"true"`
}

// newConnectionCreator returns the [flam.IDatabaseConnectionCreator] of the
// gorm connections, reading the defaults of the missing fields from the
// configuration. It is a provider registered by the bundle returned from
// [NewBundle].
func newConnectionCreator(
	args connectionCreatorArgs,
) flam.IDatabaseConnectionCreator {
	return &connectionCreator{
		config:   args.Config,
		dialects: args.Dialects,
		configs:  args.Configs,
		clock:    args.Clock,
	}
}

// connectionCreator is the [flam.IDatabaseConnectionCreator] of the
// configurations without driver or with the [Driver] driver.
type connectionCreator struct {
	config   flam.IConfig
	dialects flam.IDatabaseDialectFactory
	configs  flam.IDatabaseConfigFactory
	clock    flam.IClock
}

func (c *connectionCreator) Accept(
	bag flam.IBag,
) bool {
	driver := bag.Get(factory.ConfigFieldDriver)
	return driver == nil || driver == Driver
}

// Create opens the connection with the dialect and the gorm configuration
// of the ids of the configuration, or of their defaults, along with its
// read replicas and extra sources, retrying as configured.
func (c *connectionCreator) Create(
	bag flam.IBag,
) (flam.IDatabaseConnection, error) {
	s, err := c.read(bag)
	if err != nil {
		return nil, err
	}

	limit := max(s.delay, ConnectRetryMaxDelay)
	for attempt := 0; ; attempt++ {
		conn, err := c.open(s)
		if err == nil || attempt == s.retries {
			return conn, err
		}
		c.clock.Sleep(backoff.Delay(s.delay, limit, attempt+1))
	}
}

// connectionSettings are the values of a connection configuration, read
// by Create.
type connectionSettings struct {
	id        string
	dialectID string
	configID  string
	dialect   flam.IDatabaseDialect
	config    gorm.Config
	replicas  []flam.IDatabaseDialect
	sources   []flam.IDatabaseDialect
	policy    string
	retries   int
	delay     time.Duration
}

// read returns the values of the connection configuration, or the error
// of the first invalid field, or of the first dialect or gorm
// configuration that cannot be taken from its factory.
func (c *connectionCreator) read(
	bag flam.IBag,
) (*connectionSettings, error) {
	s := &connectionSettings{id: bag.String(factory.ConfigFieldId)}

	var ok bool
	if s.dialectID, ok = c.field(bag, ConfigFieldDialect, DefaultDialectConfigPath); !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldDialect)
	}
	if s.configID, ok = c.field(bag, ConfigFieldConfig, DefaultConfigConfigPath); !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldConfig)
	}
	replicaIDs, ok := connectionCreatorIDs(settings.Field(c.config, bag, ConfigFieldReplicas, "", nil))
	if !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldReplicas)
	}
	sourceIDs, ok := connectionCreatorIDs(settings.Field(c.config, bag, ConfigFieldSources, "", nil))
	if !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldSources)
	}
	s.policy, _ = settings.Field(c.config, bag, ConfigFieldPolicy, DefaultPolicyConfigPath, defaultPolicy).(string)
	if s.policy != PolicyRandom && s.policy != PolicyRoundRobin {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldPolicy)
	}
	if s.retries, ok = connectionCreatorCount(settings.Field(c.config, bag, ConfigFieldConnectRetries, DefaultConnectRetriesConfigPath, defaultConnectRetries)); !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldConnectRetries)
	}
	if s.delay, ok = settings.Duration(settings.Field(c.config, bag, ConfigFieldConnectRetryDelay, DefaultConnectRetryDelayConfigPath, defaultConnectRetryDelay)); !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, s.id, ConfigFieldConnectRetryDelay)
	}
	if s.retries > 0 && c.clock == nil {
		return nil, newErrNil(ErrNilClock, connectionCreatorCreateMessage, s.id)
	}

	var err error
	if s.dialect, err = c.dialects.Get(s.dialectID); err != nil {
		return nil, newErrDialect(connectionCreatorCreateMessage, s.id, s.dialectID, err)
	}
	config, err := c.configs.Get(s.configID)
	if err != nil {
		return nil, newErrConfig(connectionCreatorCreateMessage, s.id, s.configID, err)
	}
	s.config = config.Config
	if s.replicas, err = c.dialectsOf(s.id, replicaIDs); err != nil {
		return nil, err
	}
	if s.sources, err = c.dialectsOf(s.id, sourceIDs); err != nil {
		return nil, err
	}
	return s, nil
}

// open opens the connection of the settings, registering the resolver of
// its replicas and sources when it has some.
func (c *connectionCreator) open(
	s *connectionSettings,
) (flam.IDatabaseConnection, error) {
	// a copy, with maps of its own, as gorm registers the plugins in it
	gormConfig := s.config
	gormConfig.Plugins = maps.Clone(gormConfig.Plugins)
	gormConfig.ClauseBuilders = maps.Clone(gormConfig.ClauseBuilders)
	db, err := gorm.Open(s.dialect.Gorm(), &gormConfig)
	if err != nil {
		return nil, newErrOpen(connectionCreatorCreateMessage, s.id, s.dialectID, s.configID, err)
	}
	if len(s.replicas) == 0 && len(s.sources) == 0 {
		return NewConnection(s.id, db)
	}

	conn := &ownedConnection{connection: &connection{id: s.id, db: db}}
	resolver := dbresolver.Config{
		Replicas: connectionCreatorOpened(s.replicas, &conn.pools),
		Policy:   connectionCreatorPolicy(s.policy),
	}
	if len(s.sources) > 0 {
		pool := db.ConnPool
		if prepared, ok := pool.(*gorm.PreparedStmtDB); ok {
			pool = prepared.ConnPool
		}
		resolver.Sources = append(
			[]gorm.Dialector{&pooledDialector{Dialector: s.dialect.Gorm(), pool: pool}},
			connectionCreatorOpened(s.sources, &conn.pools)...,
		)
	}
	if err := db.Use(dbresolver.Register(resolver)); err != nil {
		_ = conn.Close()
		return nil, newErrResolver(connectionCreatorCreateMessage, s.id, err)
	}
	return conn, nil
}

// field returns the id held by the field of the connection configuration,
// or else by the default path, and false when it is not a non-empty
// string.
func (c *connectionCreator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
) (string, bool) {
	val := c.config.Get(defaultPath)
	if bag.Has(field) {
		val = bag.Get(field)
	}
	s, ok := val.(string)
	return s, ok && s != ""
}

// dialectsOf returns the dialects of the ids, failing with the error of
// the first one that cannot be taken from the dialect factory.
func (c *connectionCreator) dialectsOf(
	id string,
	ids []string,
) ([]flam.IDatabaseDialect, error) {
	dialects := make([]flam.IDatabaseDialect, 0, len(ids))
	for _, dialectID := range ids {
		dialect, err := c.dialects.Get(dialectID)
		if err != nil {
			return nil, newErrDialect(connectionCreatorCreateMessage, id, dialectID, err)
		}
		dialects = append(dialects, dialect)
	}
	return dialects, nil
}

// connectionCreatorIDs returns the ids of the list, a []string or a []any
// of strings, nil being an empty list, and false when it is not a list of
// non-empty strings.
func connectionCreatorIDs(
	val any,
) ([]string, bool) {
	ids, ok := settings.Strings(val)
	for _, id := range ids {
		ok = ok && id != ""
	}
	return ids, ok
}

// connectionCreatorCount returns the whole number held by the value, of
// any integer or float kind, 0 or more, and false for any other value.
func connectionCreatorCount(
	val any,
) (int, bool) {
	var n float64
	rv := reflect.ValueOf(val)
	switch {
	case rv.CanInt():
		n = float64(rv.Int())
	case rv.CanUint():
		n = float64(rv.Uint())
	case rv.CanFloat():
		n = rv.Float()
	default:
		return 0, false
	}
	return int(n), n == math.Trunc(n) && n >= 0 && n <= math.MaxInt32
}

// connectionCreatorCreateMessage is the message of the errors returned by
// Create.
const connectionCreatorCreateMessage = "unable to create database connection"
