package flam

import (
	"go.uber.org/dig"
)

type Provider interface {
	Id() string
	Register(container *dig.Container) error
}

type ConfigurableProvider interface {
	Provider

	Config(config *Bag) error
}

type BootableProvider interface {
	Provider

	Boot(container *dig.Container) error
}

type RunnableProvider interface {
	Provider

	Run(container *dig.Container) error
}

type ClosableProvider interface {
	Provider

	Close(container *dig.Container) error
}

type provider struct{}

var _ Provider = (*provider)(nil)
var _ ClosableProvider = (*provider)(nil)

func newProvider() Provider {
	return &provider{}
}

func (*provider) Id() string {
	return providerId
}

func (*provider) Register(
	container *dig.Container,
) error {
	return NewRegisterer().
		Queue(func() PubSub[string, string] { return NewPubSub[string, string]() }).
		Queue(newTimer).
		Queue(newTriggerFactory).
		Queue(newDiskFactory).
		Queue(newOsDiskCreator, dig.Group(DiskCreatorGroup)).
		Queue(newMemoryDiskCreator, dig.Group(DiskCreatorGroup)).
		Queue(newConfigRestClientGenerator).
		Queue(newConfigParserFactory).
		Queue(newJsonConfigParserCreator, dig.Group(ConfigParserCreatorGroup)).
		Queue(newYamlConfigParserCreator, dig.Group(ConfigParserCreatorGroup)).
		Queue(newConfigSourceFactory).
		Queue(newEnvConfigSourceCreator, dig.Group(ConfigSourceCreatorGroup)).
		Queue(newFileConfigSourceCreator, dig.Group(ConfigSourceCreatorGroup)).
		Queue(newObservableFileConfigSourceCreator, dig.Group(ConfigSourceCreatorGroup)).
		Queue(newDirConfigSourceCreator, dig.Group(ConfigSourceCreatorGroup)).
		Queue(newRestConfigSourceCreator, dig.Group(ConfigSourceCreatorGroup)).
		Queue(newObservableRestConfigSourceCreator, dig.Group(ConfigSourceCreatorGroup)).
		Queue(newConfig).
		Queue(func(config *config) Config { return config }).
		Queue(newConfigObserver).
		Queue(newConfigBooter).
		Queue(newFactoryConfig).
		Queue(newLogSerializerFactory).
		Queue(newStringLogSerializerCreator, dig.Group(LogSerializerCreatorGroup)).
		Queue(newJsonLogSerializerCreator, dig.Group(LogSerializerCreatorGroup)).
		Queue(newLogStreamFactory).
		Queue(newConsoleLogStreamCreator, dig.Group(LogStreamCreatorGroup)).
		Queue(newFileLogStreamCreator, dig.Group(LogStreamCreatorGroup)).
		Queue(newRotatingFileLogStreamCreator, dig.Group(LogStreamCreatorGroup)).
		Queue(newLogger).
		Queue(func(logger *logger) Logger { return logger }).
		Queue(newLogFlusher).
		Queue(newLogBooter).
		Queue(newDatabaseConfigFactory).
		Queue(newDefaultDatabaseConfigCreator, dig.Group(DatabaseConfigCreatorGroup)).
		Queue(newDatabaseDialectFactory).
		Queue(newSqliteDatabaseDialectCreator, dig.Group(DatabaseDialectCreatorGroup)).
		Queue(newMysqlDatabaseDialectCreator, dig.Group(DatabaseDialectCreatorGroup)).
		Queue(newPostgresDatabaseDialectCreator, dig.Group(DatabaseDialectCreatorGroup)).
		Queue(newDatabaseConnectionFactory).
		Queue(newDatabaseConnectionCreator).
		Queue(newMigrationPool).
		Queue(newMigrationDao).
		Queue(newMigratorLoggerFactory).
		Queue(newDefaultMigratorLoggerCreator, dig.Group(MigratorLoggerCreatorGroup)).
		Queue(newMigratorFactory).
		Queue(newDefaultMigratorCreator, dig.Group(MigratorCreatorGroup)).
		Queue(newMigratorBooter).
		Queue(newRedisConnectionFactory).
		Queue(newDefaultRedisConnectionCreator, dig.Group(RedisConnectionCreatorGroup)).
		Queue(newMiniRedisConnectionCreator, dig.Group(RedisConnectionCreatorGroup)).
		Queue(newRedisBooter).
		Queue(newCacheSerializerFactory).
		Queue(newCacheKeyGeneratorFactory).
		Queue(newCacheAdaptorFactory).
		Queue(newRedisCacheAdaptorCreator, dig.Group(CacheAdaptorCreatorGroup)).
		Queue(newTranslatorFactory).
		Queue(newEnglishTranslatorCreator, dig.Group(TranslatorCreatorGroup)).
		Queue(newValidatorParserFactory).
		Queue(newDefaultValidatorParserCreator, dig.Group(ValidatorParserCreatorGroup)).
		Queue(newValidatorErrorConverterFactory).
		Queue(newValidatorFactory).
		Queue(newDefaultValidatorCreator, dig.Group(ValidatorCreatorGroup)).
		Queue(newWatchdogLoggerFactory).
		Queue(newDefaultWatchdogLoggerCreator, dig.Group(WatchdogLoggerCreatorGroup)).
		Queue(newKennel).
		Queue(func(kennel *kennel) Kennel { return kennel }).
		Run(container)
}

func (*provider) Config(
	config *Bag,
) error {
	_ = config.Set(PathConfigBoot, DefaultConfigBoot)
	_ = config.Set(PathConfigObserverFrequency, DefaultConfigObserverFrequency)
	_ = config.Set(PathConfigDefaultFileParserId, DefaultConfigFileParserId)
	_ = config.Set(PathConfigDefaultFileDiskId, DefaultConfigFileDiskId)
	_ = config.Set(PathConfigDefaultRestParserId, DefaultConfigRestParserId)
	_ = config.Set(PathConfigDefaultRestConfigPath, DefaultConfigRestConfigPath)
	_ = config.Set(PathConfigDefaultRestTimestampPath, DefaultConfigRestTimestampPath)
	_ = config.Set(PathConfigDefaultPriority, DefaultConfigPriority)

	_ = config.Set(PathLogBoot, DefaultLogBoot)
	_ = config.Set(PathLogFlusherFrequency, DefaultLogFlusherFrequency)
	_ = config.Set(PathLogDefaultLevel, DefaultLogLevel)
	_ = config.Set(PathLogDefaultSerializerId, DefaultLogSerializerId)
	_ = config.Set(PathLogDefaultDiskId, DefaultLogDiskId)

	_ = config.Set(PathDatabaseDefaultSqliteHost, DefaultDatabaseSqliteHost)
	_ = config.Set(PathDatabaseDefaultMySqlProtocol, DefaultDatabaseMySqlProtocol)
	_ = config.Set(PathDatabaseDefaultMySqlHost, DefaultDatabaseMySqlHost)
	_ = config.Set(PathDatabaseDefaultMySqlPort, DefaultDatabaseMySqlPort)
	_ = config.Set(PathDatabaseDefaultPostgresHost, DefaultDatabasePostgresHost)
	_ = config.Set(PathDatabaseDefaultPostgresPort, DefaultDatabasePostgresPort)
	_ = config.Set(PathDatabaseDefaultDialectId, DefaultDatabaseDialectId)
	_ = config.Set(PathDatabaseDefaultConfigId, DefaultDatabaseConfigId)

	_ = config.Set(PathMigratorBoot, DefaultMigratorBoot)
	_ = config.Set(PathMigratorDefaultConnectionId, DefaultMigratorConnectionId)
	_ = config.Set(PathMigratorDefaultLoggerId, DefaultMigratorLoggerId)
	_ = config.Set(PathMigratorDefaultLoggerChannel, DefaultMigratorLoggerChannel)
	_ = config.Set(PathMigratorDefaultLoggerStartLevel, DefaultMigratorLoggerStartLevel)
	_ = config.Set(PathMigratorDefaultLoggerErrorLevel, DefaultMigratorLoggerErrorLevel)
	_ = config.Set(PathMigratorDefaultLoggerDoneLevel, DefaultMigratorLoggetDoneLevel)

	_ = config.Set(PathRedisMiniBoot, DefaultRedisMiniBoot)
	_ = config.Set(PathRedisDefaultHost, DefaultRedisHost)
	_ = config.Set(PathRedisDefaultPort, DefaultRedisPort)
	_ = config.Set(PathRedisDefaultPassword, DefaultRedisPassword)
	_ = config.Set(PathRedisDefaultDatabase, DefaultRedisDatabase)

	_ = config.Set(PathCacheDefaultKeyGeneratorId, DefaultCacheKeyGeneratorId)
	_ = config.Set(PathCacheDefaultSerializerId, DefaultCacheSerializerId)

	_ = config.Set(PathValidatorDefaultAnnotation, DefaultValidatorAnnotation)
	_ = config.Set(PathValidatorDefaultTranslatorId, DefaultValidatorTranslatorId)
	_ = config.Set(PathValidatorDefaultParserId, DefaultValidatorParserId)
	_ = config.Set(PathValidatorDefaultErrorConverterId, DefaultValidatorErrorConverterId)

	_ = config.Set(PathKennelRun, DefaultKennelRun)
	_ = config.Set(PathWatchdogDefaultLoggerId, DefaultWatchdogLoggerId)
	_ = config.Set(PathWatchdogDefaultLoggerChannel, DefaultWatchdogLoggerChannel)
	_ = config.Set(PathWatchdogDefaultLoggerStartLevel, DefaultWatchdogLoggerStartLevel)
	_ = config.Set(PathWatchdogDefaultLoggerErrorLevel, DefaultWatchdogLoggerErrorLevel)
	_ = config.Set(PathWatchdogDefaultLoggerDoneLevel, DefaultWatchdogLoggerDoneLevel)

	return nil
}

func (provider *provider) Boot(
	container *dig.Container,
) error {
	return NewExecutor().
		Queue(provider.bootConfig).
		Queue(provider.bootLog).
		Queue(provider.bootMigrator).
		Queue(provider.bootRedis).
		Run(container)
}

func (provider *provider) Run(
	container *dig.Container,
) error {
	return NewExecutor().
		Queue(provider.runKennel).
		Run(container)
}

func (provider *provider) bootConfig(
	configBooter *configBooter,
	configObserver *configObserver,
) error {
	var e error
	exec := func(f func() error) bool {
		e = f()
		return e == nil
	}

	_ = exec(configBooter.Boot) &&
		exec(configObserver.Boot)

	return e
}

func (provider *provider) bootLog(
	logBooter *logBooter,
	logFlusher *logFlusher,
) error {
	var e error
	exec := func(f func() error) bool {
		e = f()
		return e == nil
	}

	_ = exec(logBooter.Boot) &&
		exec(logFlusher.Boot)

	return e
}

func (provider *provider) bootMigrator(
	migratorBooter *migratorBooter,
) error {
	return migratorBooter.Boot()
}

func (provider *provider) bootRedis(
	redisBooter *redisBooter,
) error {
	return redisBooter.Boot()
}

func (provider *provider) runKennel(
	kennel *kennel,
) error {
	return kennel.run()
}

func (provider *provider) Close(
	container *dig.Container,
) error {
	return NewExecutor().
		Queue(provider.closeKennel).
		Queue(provider.closeWatchdogLoggerFactory).
		Queue(provider.closeValidatorFactory).
		Queue(provider.closeValidatorErrorConverterFactory).
		Queue(provider.closeValidatorParserFactory).
		Queue(provider.closeTranslatorFactory).
		Queue(provider.closeCacheAdaptorFactory).
		Queue(provider.closeCacheSerializerFactory).
		Queue(provider.closeCacheKeyGeneratorFactory).
		Queue(provider.closeRedisConnectionFactory).
		Queue(provider.closeRedisMini).
		Queue(provider.closeMigratorFactory).
		Queue(provider.closeMigratorLoggerFactory).
		Queue(provider.closeDatabaseConnectionFactory).
		Queue(provider.closeDatabaseDialectFactory).
		Queue(provider.closeLogFlusher).
		Queue(provider.closeLogger).
		Queue(provider.closeLogStreamFactory).
		Queue(provider.closeLogSerializerFactory).
		Queue(provider.closeConfigObserver).
		Queue(provider.closeConfigSourceFactory).
		Queue(provider.closeConfigParserFactory).
		Queue(provider.closeDiskFactory).
		Run(container)
}

func (*provider) closeWatchdogLoggerFactory(
	watchdogLoggerFactory WatchdogLoggerFactory,
) error {
	return watchdogLoggerFactory.Close()
}

func (*provider) closeKennel(
	kennel Kennel,
) error {
	return kennel.Close()
}

func (*provider) closeValidatorFactory(
	validatorFactory ValidatorFactory,
) error {
	return validatorFactory.Close()
}

func (*provider) closeValidatorErrorConverterFactory(
	validatorErrorConverterFactory ValidatorErrorConverterFactory,
) error {
	return validatorErrorConverterFactory.Close()
}

func (*provider) closeValidatorParserFactory(
	validatorParserFactory ValidatorParserFactory,
) error {
	return validatorParserFactory.Close()
}

func (*provider) closeTranslatorFactory(
	translatorFactory TranslatorFactory,
) error {
	return translatorFactory.Close()
}

func (*provider) closeCacheKeyGeneratorFactory(
	cacheKeyGeneratorFactory CacheKeyGeneratorFactory,
) error {
	return cacheKeyGeneratorFactory.Close()
}

func (*provider) closeCacheSerializerFactory(
	cacheSerializerFactory CacheSerializerFactory,
) error {
	return cacheSerializerFactory.Close()
}

func (*provider) closeCacheAdaptorFactory(
	cacheAdaptorFactory CacheAdaptorFactory,
) error {
	return cacheAdaptorFactory.Close()
}

func (*provider) closeRedisConnectionFactory(
	redisConnectionFactory RedisConnectionFactory,
) error {
	return redisConnectionFactory.Close()
}

func (*provider) closeRedisMini(
	redisBooter *redisBooter,
) error {
	return redisBooter.Close()
}

func (*provider) closeMigratorFactory(
	migratorFactory MigratorFactory,
) error {
	return migratorFactory.Close()
}

func (*provider) closeMigratorLoggerFactory(
	migratorLoggerFactory MigratorLoggerFactory,
) error {
	return migratorLoggerFactory.Close()
}

func (*provider) closeDatabaseConnectionFactory(
	databaseConnectionFactory DatabaseConnectionFactory,
) error {
	return databaseConnectionFactory.Close()
}

func (*provider) closeDatabaseDialectFactory(
	databaseDialectFactory DatabaseDialectFactory,
) error {
	return databaseDialectFactory.Close()
}

func (provider *provider) closeLogFlusher(
	logFlusher *logFlusher,
) error {
	return logFlusher.Close()
}

func (provider *provider) closeLogger(
	logger *logger,
) error {
	return logger.Close()
}

func (provider *provider) closeLogStreamFactory(
	logStreamFactory LogStreamFactory,
) error {
	return logStreamFactory.Close()
}

func (provider *provider) closeLogSerializerFactory(
	logSerializerFactory LogSerializerFactory,
) error {
	return logSerializerFactory.Close()
}

func (provider *provider) closeConfigObserver(
	configObserver *configObserver,
) error {
	return configObserver.Close()
}

func (*provider) closeDiskFactory(
	factory DiskFactory,
) error {
	return factory.Close()
}

func (*provider) closeConfigParserFactory(
	factory ConfigParserFactory,
) error {
	return factory.Close()
}

func (*provider) closeConfigSourceFactory(
	factory ConfigSourceFactory,
) error {
	return factory.Close()
}
