package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ValidatorFactory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.Empty(t, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of added validators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorAlphaMock := mocks.NewMockValidator(ctrl)
		validatorZuluMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			require.NoError(t, factory.Store("alpha", validatorAlphaMock))
			require.NoError(t, factory.Store("zulu", validatorZuluMock))

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
		}))
	})

	t.Run("should return a sorted list of ids of combined added validators and config defined validators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		validatorCharlieMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			require.NoError(t, factory.Store("charlie", validatorCharlieMock))

			assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
		}))
	})
}

func Test_ValidatorFactory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.Empty(t, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of generated validators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"zulu": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"},
			"alpha": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		parserMock := mocks.NewMockValidatorParser(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			require.NoError(t, factory.Store("my_parser", parserMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of added validators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorMock1 := mocks.NewMockValidator(ctrl)
		validatorMock2 := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			require.NoError(t, factory.Store("my_validator_1", validatorMock1))
			require.NoError(t, factory.Store("my_validator_2", validatorMock2))

			assert.Equal(t, []string{"my_validator_1", "my_validator_2"}, factory.Stored())
		}))
	})

	t.Run("should return a sorted list of a combination of added and generated validators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"zulu": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"},
			"alpha": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		validatorMock1 := mocks.NewMockValidator(ctrl)
		validatorMock2 := mocks.NewMockValidator(ctrl)

		parserMock := mocks.NewMockValidatorParser(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			require.NoError(t, factory.Store("my_parser", parserMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			instance, e := factory.Get("zulu")
			require.NotNil(t, instance)
			require.NoError(t, e)

			instance, e = factory.Get("alpha")
			require.NotNil(t, instance)
			require.NoError(t, e)

			require.NoError(t, factory.Store("my_validator_1", validatorMock1))
			require.NoError(t, factory.Store("my_validator_2", validatorMock2))

			assert.Equal(t, []string{"alpha", "my_validator_1", "my_validator_2", "zulu"}, factory.Stored())
		}))
	})
}

func Test_ValidatorFactory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := flam.Bag{}
	_ = config.Set(flam.PathValidators, flam.Bag{
		"ny_validator_1": flam.Bag{
			"driver": flam.ValidatorDriverDefault}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	validatorMock := mocks.NewMockValidator(ctrl)

	require.NoError(t, app.Boot())

	require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
		require.NoError(t, factory.Store("ny_validator_2", validatorMock))

		testCases := []struct {
			name     string
			id       string
			expected bool
		}{
			{
				name:     "entry in config",
				id:       "ny_validator_1",
				expected: true},
			{
				name:     "manually added entry",
				id:       "ny_validator_2",
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

func Test_ValidatorFactory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			got, e := factory.Get("nonexistent")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return config error if driver is not present in config", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			got, e := factory.Get("my_validator")
			assert.Nil(t, got)
			assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return the same previously retrieved validator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver":    flam.ValidatorDriverDefault,
				"parser_id": "my_parser"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		parserMock := mocks.NewMockValidatorParser(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorParserFactory) {
			require.NoError(t, factory.Store("my_parser", parserMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			got, e := factory.Get("my_validator")
			require.NotNil(t, got)
			require.NoError(t, e)

			got3, e := factory.Get("my_validator")
			require.Same(t, got, got3)
			require.NoError(t, e)
		}))
	})
}

func Test_ValidatorFactory_Store(t *testing.T) {
	t.Run("should return nil reference if validator is nil", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.ErrorIs(t, factory.Store("my_validator", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should return duplicate resource error if validator reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathValidators, flam.Bag{
			"my_validator": flam.Bag{
				"driver": flam.ValidatorDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		validatorMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.ErrorIs(t, factory.Store("my_validator", validatorMock), flam.ErrDuplicateResource)
		}))
	})

	t.Run("should return nil error if validator has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.NoError(t, factory.Store("my_validator", validatorMock))
		}))
	})

	t.Run("should return duplicate resource if validator has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.NoError(t, factory.Store("my_validator", validatorMock))
			assert.ErrorIs(t, factory.Store("my_validator", validatorMock), flam.ErrDuplicateResource)
		}))
	})
}

func Test_ValidatorFactory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the validator is not stored", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			assert.ErrorIs(t, factory.Remove("my_validator"), flam.ErrUnknownResource)
		}))
	})

	t.Run("should remove validator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorMock := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			require.NoError(t, factory.Store("my_validator", validatorMock))

			assert.NoError(t, factory.Remove("my_validator"))

			assert.Empty(t, factory.Stored())
		}))
	})
}

func Test_ValidatorFactory_RemoveAll(t *testing.T) {
	t.Run("should correctly remove all stored validators", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		validatorMock1 := mocks.NewMockValidator(ctrl)
		validatorMock2 := mocks.NewMockValidator(ctrl)

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ValidatorFactory) {
			require.NoError(t, factory.Store("my_validator_1", validatorMock1))
			require.NoError(t, factory.Store("my_validator_2", validatorMock2))

			assert.NoError(t, factory.RemoveAll())

			assert.Empty(t, factory.Stored())
		}))
	})
}
