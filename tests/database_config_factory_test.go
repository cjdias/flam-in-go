package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go"
)

func Test_DatabaseConfigFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added configs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			require.NoError(t, factory.Store("alpha", &gorm.Config{}))
			require.NoError(t, factory.Store("zulu", &gorm.Config{}))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added configs and config defined configs", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			require.NoError(t, factory.Store("charlie", &gorm.Config{}))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_DatabaseConfigFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated configs", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault},
			"alpha": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added configs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			require.NoError(t, factory.Store("my_config_1", &gorm.Config{}))
			require.NoError(t, factory.Store("my_config_2", &gorm.Config{}))

			assert.Equal(t, []string{"my_config_1", "my_config_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated configs", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"zulu": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault},
			"alpha": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_config_1", &gorm.Config{}))
			require.NoError(t, factory.Store("my_config_2", &gorm.Config{}))

			assert.Equal(t, []string{"alpha", "my_config_1", "my_config_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_DatabaseConfigFactory_Has(t *testing.T) {
	config := flam.Bag{}
	_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
		"ny_config_1": flam.Bag{
			"driver": flam.DatabaseConfigDriverDefault}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
		require.NoError(t, factory.Store("ny_config_2", &gorm.Config{}))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_config_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_config_2",
				expected: true},
			{
				name:     "non-existent entry",
				id:       "nonexistent",
				expected: false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, factory.Has(tc.id))
			})
		}
	}))
}

func Test_DatabaseConfigFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("my_config")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			got, e := factory.Get("my_config")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_config")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_DatabaseConfigFactory_Store(t *testing.T) {
	t.Run("should return nil reference if config is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.ErrorIs(t, factory.Store("my_config", (*gorm.Config)(nil)), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if config reference exists in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathDatabaseConfigs, flam.Bag{
			"my_config": flam.Bag{
				"driver": flam.DatabaseConfigDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.ErrorIs(t, factory.Store("my_config", &gorm.Config{}), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if config has been stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.NoError(t, factory.Store("my_config", &gorm.Config{}))
		}))
	})

	t.Run("should return duplicate resource if config has already been stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.NoError(t, factory.Store("my_config", &gorm.Config{}))
			assert.ErrorIs(t, factory.Store("my_config", &gorm.Config{}), flam.ErrDuplicateResource)
		}))
	})
}

func Test_DatabaseConfigFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the config is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			assert.ErrorIs(t, factory.Remove("my_config"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove config", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			require.NoError(t, factory.Store("my_config", &gorm.Config{}))

			assert.NoError(t, factory.Remove("my_config"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_DatabaseConfigFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored configs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.DatabaseConfigFactory) {
			require.NoError(t, factory.Store("my_config_1", &gorm.Config{}))
			require.NoError(t, factory.Store("my_config_2", &gorm.Config{}))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
