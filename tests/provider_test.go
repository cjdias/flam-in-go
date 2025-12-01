package tests

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_Provider_Id(t *testing.T) {
	t.Run("should return the expected default provider id", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.True(t, app.HasProvider("flam.provider"))
	})
}

func Test_Provider_Register(t *testing.T) {
	t.Run("should provider a pubsub instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.PubSub[string, string]) {}))
	})

	t.Run("should provider a timer instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.Timer) {}))
	})

	t.Run("should provider a trigger factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.TriggerFactory) {}))
	})

	t.Run("should provider a disk factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.DiskFactory) {}))
	})

	t.Run("should provider a 'os' disk instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverOS}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk, e := factory.Get("my_disk")
			assert.NotNil(t, disk)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'memory' disk instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			generated, e := factory.Get("my_disk")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a config rest client generator instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.ConfigRestClientGenerator) {}))
	})

	t.Run("should provider a config parser factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.ConfigParserFactory) {}))
	})

	t.Run("should provider a 'json' config parser instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			generated, e := factory.Get("my_parser")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'yaml' config parser instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			generated, e := factory.Get("my_parser")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a config source factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.ConfigSourceFactory) {}))
	})

	t.Run("should provider a 'env' config source instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			generated, e := factory.Get("my_source")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'file' config source instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverFile,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "my_file.json"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk, e := factory.Get("my_disk")
			require.NotNil(t, disk)
			require.NoError(t, e)

			file, e := disk.Create("my_file.json")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.Write([]byte("{}"))
		}))

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			generated, e := factory.Get("my_source")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'observable file' config source instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "my_file.json"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk, e := factory.Get("my_disk")
			require.NotNil(t, disk)
			require.NoError(t, e)

			file, e := disk.Create("my_file.json")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.Write([]byte("{}"))
		}))

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			generated, e := factory.Get("my_source")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'dir' config source instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"parser_id": "my_parser",
				"path":      "my_dir"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk, e := factory.Get("my_disk")
			require.NotNil(t, disk)
			require.NoError(t, e)

			e = disk.Mkdir("my_dir", fs.FileMode(0644))
			require.NoError(t, e)
		}))

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			generated, e := factory.Get("my_source")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'rest' config source instance", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://uri/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configRestClient := mocks.NewMockConfigRestClient(ctrl)
		configRestClient.EXPECT().Do(gomock.Any()).Return(&http.Response{
			Body: io.NopCloser(bytes.NewReader(
				[]byte(`{"config": {}}`)))}, nil)
		configRestClientGenerator := mocks.NewMockConfigRestClientGenerator(ctrl)
		configRestClientGenerator.EXPECT().Create().Return(configRestClient, nil)
		require.NoError(t, app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return configRestClientGenerator
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			generated, e := factory.Get("my_source")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'observable rest' config source instance", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://uri/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		configRestClient := mocks.NewMockConfigRestClient(ctrl)
		configRestClient.EXPECT().Do(gomock.Any()).Return(&http.Response{
			Body: io.NopCloser(bytes.NewReader([]byte(
				fmt.Sprintf(`{"config": {}, "timestamp": "%s"}`, time.Now().Format(time.RFC3339)))))}, nil)
		configRestClientGenerator := mocks.NewMockConfigRestClientGenerator(ctrl)
		configRestClientGenerator.EXPECT().Create().Return(configRestClient, nil)
		require.NoError(t, app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return configRestClientGenerator
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			generated, e := factory.Get("my_source")
			assert.NotNil(t, generated)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a config instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.Config) {}))
	})

	t.Run("should provider a factory config instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.FactoryConfig) {}))
	})

	t.Run("should provider a log serializer factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {}))
	})

	t.Run("should provider a 'string' log serializer instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverString}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			generator, e := factory.Get("my_serializer")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'json' log serializer instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			generator, e := factory.Get("my_serializer")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a log stream factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {}))
	})

	t.Run("should provider a 'console' log stream instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serializer"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			generator, e := factory.Get("my_stream")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'file' log stream instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverFile,
				"disk_id":       "my_disk",
				"serializer_id": "my_serializer",
				"path":          "my_file.log"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			generator, e := factory.Get("my_stream")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'rotating file' log stream instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDisks, flam.Bag{
			"my_disk": flam.Bag{
				"driver": flam.DiskDriverMemory}})
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverRotatingFile,
				"disk_id":       "my_disk",
				"serializer_id": "my_serializer",
				"path":          "my_file-%s.log"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			generator, e := factory.Get("my_stream")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a logger instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.Logger) {}))
	})

	t.Run("should provider a database config factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.DatabaseConfigFactory) {}))
	})

	t.Run("should provider a default database config instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			generator, e := factory.Get("my_config")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a database dialect factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.DatabaseDialectFactory) {}))
	})

	t.Run("should provider a 'sqlite' database dialect instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite,
				"host":   ":memory:"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			generator, e := factory.Get("my_dialect")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'mysql' database dialect instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver":   flam.DatabaseDialectDriverMySql,
				"username": "user",
				"password": "password",
				"schema":   "db"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			generator, e := factory.Get("my_dialect")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'postgres' database dialect instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver":   flam.DatabaseDialectDriverPostgres,
				"username": "user",
				"password": "password",
				"schema":   "db"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			generator, e := factory.Get("my_dialect")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a database connection factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.DatabaseConnectionFactory) {}))
	})

	t.Run("should provider a configured database connection instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite,
				"host":   ":memory:"}})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"my_connection": flam.Bag{
				"config_id":  "my_config",
				"dialect_id": "my_dialect"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			generator, e := factory.Get("my_connection")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a migrator logger factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.MigratorLoggerFactory) {}))
	})

	t.Run("should provider a default migrator logger instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.MigratorLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			generator, e := factory.Get("my_logger")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a migrator factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.MigratorFactory) {}))
	})

	t.Run("should provider a default migrator instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite,
				"host":   ":memory:"}})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"my_connection": flam.Bag{
				"config_id":  "my_config",
				"dialect_id": "my_dialect"}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "primary"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			generator, e := factory.Get("my_migrator")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a redis connection factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.RedisConnectionFactory) {}))
	})

	t.Run("should provider a 'default' redis connection instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			generator, e := factory.Get("my_connection")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a 'mini' redis connection instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverMini,
				"host":   "localhost"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			generator, e := factory.Get("my_connection")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a cache serializer factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.CacheSerializerFactory) {}))
	})

	t.Run("should provider a cache key generator factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.CacheKeyGeneratorFactory) {}))
	})

	t.Run("should provider a cache adaptor factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.CacheAdaptorFactory) {}))
	})

	t.Run("should provider a 'redis' cache adaptor instance", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_redis_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverDefault}})
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_redis_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			e := factory.Store("my_serializer", cacheSerializerMock)
			require.NoError(t, e)
		}))

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			e := factory.Store("my_key_generator", cacheKeyGeneratorMock)
			require.NoError(t, e)
		}))

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			generator, e := factory.Get("my_adaptor")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a translator factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.TranslatorFactory) {}))
	})

	t.Run("should provider a 'english' translator instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			generator, e := factory.Get("my_translator")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a validator parser factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.ValidatorParserFactory) {}))
	})

	t.Run("should provider a 'default' validator parser instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			generator, e := factory.Get("my_parser")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a validator error converter factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.ValidatorErrorConverterFactory) {}))
	})

	t.Run("should provider a validator factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.ValidatorFactory) {}))
	})

	t.Run("should provider a 'default' validator instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathTranslators, flam.Bag{
			"my_translator": flam.Bag{
				"driver": flam.TranslatorDriverEnglish}})
		_ = config.Set(flam.PathValidatorParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver":        flam.ValidatorParserDriverDefault,
				"translator_id": "my_translator"}})
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			generator, e := factory.Get("my_validator")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a watchdog logger factory instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.WatchdogLoggerFactory) {}))
	})

	t.Run("should provider a 'default' watchdog logger instance", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			generator, e := factory.Get("my_logger")
			assert.NotNil(t, generator)
			assert.NoError(t, e)
		}))
	})

	t.Run("should provider a kennel instance", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(flam.Kennel) {}))
	})
}

func Test_Provider_Config(t *testing.T) {
	t.Run("should load all default configs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.Equal(t, config.Get(flam.PathConfigBoot), flam.DefaultConfigBoot)
			assert.Equal(t, config.Get(flam.PathConfigObserverFrequency), flam.DefaultConfigObserverFrequency)
			assert.Equal(t, config.Get(flam.PathConfigDefaultFileParserId), flam.DefaultConfigFileParserId)
			assert.Equal(t, config.Get(flam.PathConfigDefaultFileDiskId), flam.DefaultConfigFileDiskId)
			assert.Equal(t, config.Get(flam.PathConfigDefaultRestParserId), flam.DefaultConfigRestParserId)
			assert.Equal(t, config.Get(flam.PathConfigDefaultRestConfigPath), flam.DefaultConfigRestConfigPath)
			assert.Equal(t, config.Get(flam.PathConfigDefaultRestTimestampPath), flam.DefaultConfigRestTimestampPath)
			assert.Equal(t, config.Get(flam.PathConfigDefaultPriority), flam.DefaultConfigPriority)

			assert.Equal(t, config.Get(flam.PathLogBoot), flam.DefaultLogBoot)
			assert.Equal(t, config.Get(flam.PathLogFlusherFrequency), flam.DefaultLogFlusherFrequency)
			assert.Equal(t, config.Get(flam.PathLogDefaultLevel), flam.DefaultLogLevel)
			assert.Equal(t, config.Get(flam.PathLogDefaultSerializerId), flam.DefaultLogSerializerId)
			assert.Equal(t, config.Get(flam.PathLogDefaultDiskId), flam.DefaultLogDiskId)

			assert.Equal(t, config.Get(flam.PathDatabaseDefaultSqliteHost), flam.DefaultDatabaseSqliteHost)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultMySqlProtocol), flam.DefaultDatabaseMySqlProtocol)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultMySqlHost), flam.DefaultDatabaseMySqlHost)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultMySqlPort), flam.DefaultDatabaseMySqlPort)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultPostgresHost), flam.DefaultDatabasePostgresHost)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultPostgresPort), flam.DefaultDatabasePostgresPort)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultDialectId), flam.DefaultDatabaseDialectId)
			assert.Equal(t, config.Get(flam.PathDatabaseDefaultConfigId), flam.DefaultDatabaseConfigId)

			assert.Equal(t, config.Get(flam.PathMigratorBoot), flam.DefaultMigratorBoot)
			assert.Equal(t, config.Get(flam.PathMigratorDefaultConnectionId), flam.DefaultMigratorConnectionId)
			assert.Equal(t, config.Get(flam.PathMigratorDefaultLoggerId), flam.DefaultMigratorLoggerId)
			assert.Equal(t, config.Get(flam.PathMigratorDefaultLoggerChannel), flam.DefaultMigratorLoggerChannel)
			assert.Equal(t, config.Get(flam.PathMigratorDefaultLoggerStartLevel), flam.DefaultMigratorLoggerStartLevel)
			assert.Equal(t, config.Get(flam.PathMigratorDefaultLoggerErrorLevel), flam.DefaultMigratorLoggerErrorLevel)
			assert.Equal(t, config.Get(flam.PathMigratorDefaultLoggerDoneLevel), flam.DefaultMigratorLoggetDoneLevel)

			assert.Equal(t, config.Get(flam.PathRedisMiniBoot), flam.DefaultRedisMiniBoot)
			assert.Equal(t, config.Get(flam.PathRedisDefaultHost), flam.DefaultRedisHost)
			assert.Equal(t, config.Get(flam.PathRedisDefaultPort), flam.DefaultRedisPort)
			assert.Equal(t, config.Get(flam.PathRedisDefaultPassword), flam.DefaultRedisPassword)
			assert.Equal(t, config.Get(flam.PathRedisDefaultDatabase), flam.DefaultRedisDatabase)

			assert.Equal(t, config.Get(flam.PathCacheDefaultKeyGeneratorId), flam.DefaultCacheKeyGeneratorId)
			assert.Equal(t, config.Get(flam.PathCacheDefaultSerializerId), flam.DefaultCacheSerializerId)

			assert.Equal(t, config.Get(flam.PathValidatorDefaultAnnotation), flam.DefaultValidatorAnnotation)
			assert.Equal(t, config.Get(flam.PathValidatorDefaultTranslatorId), flam.DefaultValidatorTranslatorId)
			assert.Equal(t, config.Get(flam.PathValidatorDefaultParserId), flam.DefaultValidatorParserId)
			assert.Equal(t, config.Get(flam.PathValidatorDefaultErrorConverterId), flam.DefaultValidatorErrorConverterId)

			assert.Equal(t, config.Get(flam.PathKennelRun), flam.DefaultKennelRun)
			assert.Equal(t, config.Get(flam.PathWatchdogDefaultLoggerId), flam.DefaultWatchdogLoggerId)
			assert.Equal(t, config.Get(flam.PathWatchdogDefaultLoggerChannel), flam.DefaultWatchdogLoggerChannel)
			assert.Equal(t, config.Get(flam.PathWatchdogDefaultLoggerStartLevel), flam.DefaultWatchdogLoggerStartLevel)
			assert.Equal(t, config.Get(flam.PathWatchdogDefaultLoggerErrorLevel), flam.DefaultWatchdogLoggerErrorLevel)
			assert.Equal(t, config.Get(flam.PathWatchdogDefaultLoggerDoneLevel), flam.DefaultWatchdogLoggerDoneLevel)
		}))
	})
}

func Test_Provider_Boot(t *testing.T) {
	t.Run("should not boot config sources if is not flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.NotContains(t, factory.Stored(), "my_source")
		}))
	})

	t.Run("should boot config sources if is flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverEnv}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.Contains(t, factory.Stored(), "my_source")
		}))
	})

	t.Run("should not boot config observer if is not flagged to do so", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.False(t, config.HasObserver("flam.config", flam.PathConfigObserverFrequency))
		}))
	})

	t.Run("should boot config observer if is flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.True(t, config.HasObserver("flam.config", flam.PathConfigObserverFrequency))
		}))
	})

	t.Run("should not boot log streams if is not flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serialzier": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serialzier"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.NotContains(t, factory.Stored(), "my_stream")
		}))
	})

	t.Run("should boot log streams if is flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)
		_ = config.Set(flam.PathLogSerializers, flam.Bag{
			"my_serialzier": flam.Bag{
				"driver": flam.LogSerializerDriverJson}})
		_ = config.Set(flam.PathLogStreams, flam.Bag{
			"my_stream": flam.Bag{
				"driver":        flam.LogStreamDriverConsole,
				"serializer_id": "my_serialzier"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.Contains(t, factory.Stored(), "my_stream")
		}))
	})

	t.Run("should not boot log flusher if is not flagged to do so", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.False(t, config.HasObserver("flam.log", flam.PathLogFlusherFrequency))
		}))
	})

	t.Run("should boot log flusher if is flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.True(t, config.HasObserver("flam.log", flam.PathLogFlusherFrequency))
		}))
	})

	t.Run("should not boot migrators if is not flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver": flam.MigratorDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.NotContains(t, factory.Stored(), "my_migrator")
		}))
	})

	t.Run("should boot migrators if is flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorBoot, true)
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})
		_ = config.Set(flam.PathDatabaseDialects, flam.Bag{
			"my_dialect": flam.Bag{
				"driver": flam.DatabaseDialectDriverSqlite,
				"host":   ":memory:"}})
		_ = config.Set(flam.PathDatabaseConnections, flam.Bag{
			"my_connection": flam.Bag{
				"config_id":  "my_config",
				"dialect_id": "my_dialect"}})
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver":        flam.MigratorDriverDefault,
				"connection_id": "my_connection",
				"group":         "migration_group"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.Contains(t, factory.Stored(), "my_migrator")
		}))
	})

	t.Run("should not boot mini redis if is not flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverMini}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.Nil(t, config.Get(fmt.Sprintf("%s.my_connection.host", flam.PathRedisConnections)))
		}))
	})

	t.Run("should boot mini redis if is flagged to do so", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisMiniBoot, true)
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": flam.RedisConnectionDriverMini}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.NotNil(t, config.Get(fmt.Sprintf("%s.my_connection.host", flam.PathRedisConnections)))
		}))
	})
}

func Test_Provider_Run(t *testing.T) {
	t.Run("should not run the kennel if is not flagged to do so", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"my_process": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should run the kennel if is flagged to do so", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"my_process": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("my_process")
		processMock.EXPECT().Run().Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})
}

func Test_Provider_Close(t *testing.T) {
	t.Run("should close the kennel", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"my_process": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("my_process")
		processMock.EXPECT().Run().Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())
		require.NoError(t, app.Run())
		require.NoError(t, app.Close())
	})

	t.Run("should close the watchdog logger factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		watchdogLoggerMock := mocks.NewMockWatchdogLogger(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			require.NoError(t, factory.Store("my_instance", watchdogLoggerMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.WatchdogLoggerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the validator factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			require.NoError(t, factory.Store("my_instance", validatorMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the validator error converter factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorErrorConverterMock := mocks.NewMockValidatorErrorConverter(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			require.NoError(t, factory.Store("my_instance", validatorErrorConverterMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorErrorConverterFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the validator parser factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorParserMock := mocks.NewMockValidatorParser(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			require.NoError(t, factory.Store("my_instance", validatorParserMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the translator factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		translatorMock := mocks.NewMockTranslator(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			require.NoError(t, factory.Store("my_instance", translatorMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.TranslatorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the cache adaptor factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheAdaptorMock := mocks.NewMockCacheAdaptor(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			require.NoError(t, factory.Store("my_instance", cacheAdaptorMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the cache serializer factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_instance", cacheSerializerMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the cache key generator factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_instance", cacheKeyGeneratorMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the redis connection factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_instance", redisConnectionMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the mini redis", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathRedisMiniBoot, true)

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Close())
	})

	t.Run("should close the migrator factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_instance", migratorMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the migrator logger factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorLoggerMock := mocks.NewMockMigratorLogger(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			require.NoError(t, factory.Store("my_instance", migratorLoggerMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.MigratorLoggerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the database connection factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseConnectionMock := mocks.NewMockDatabaseConnection(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			require.NoError(t, factory.Store("my_instance", databaseConnectionMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConnectionFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the database dialect factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		databaseDialectMock := mocks.NewMockDatabaseDialect(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			require.NoError(t, factory.Store("my_instance", databaseDialectMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseDialectFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the log flusher and logger", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathLogBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Close())
	})

	t.Run("should close the log stream factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logStreamMock := mocks.NewMockLogStream(ctrl)
		logStreamMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			require.NoError(t, factory.Store("my_instance", logStreamMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogStreamFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the log serializer factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		logSerializerMock := mocks.NewMockLogSerializer(ctrl)
		logSerializerMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			require.NoError(t, factory.Store("my_instance", logSerializerMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the config observer", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Close())
	})

	t.Run("should close the config source factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().GetPriority().Return(0)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{})
		configSourceMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			require.NoError(t, factory.Store("my_instance", configSourceMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the config parser factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		configParserMock := mocks.NewMockConfigParser(ctrl)
		configParserMock.EXPECT().Close().Return(nil)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			require.NoError(t, factory.Store("my_instance", configParserMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should close the disk factory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		diskMock := mocks.NewMockDisk(ctrl)

		require.NoError(t, app.Boot())
		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_instance", diskMock))
		}))

		assert.NoError(t, app.Close())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})
}
