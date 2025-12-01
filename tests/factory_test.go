package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

type testResource struct{}

func Test_Factory_NewFactory(t *testing.T) {
	t.Run("should return proper error when config is nil", func(t *testing.T) {
		factory, e := flam.NewFactory[flam.FactoryResource](nil, nil, nil, "path")
		assert.Nil(t, factory)
		assert.ErrorIs(t, e, flam.ErrNilReference)
	})

	t.Run("should return valid factory if every is ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		assert.NotNil(t, factory)
		assert.NoError(t, e)
	})
}

func Test_Factory_Close(t *testing.T) {
	t.Run("should correctly close closable resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		closerMock := mocks.NewMockReadCloser(ctrl)
		closerMock.EXPECT().Close().Return(nil)

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", closerMock))

		assert.NoError(t, factory.Close())
	})

	t.Run("should return closing errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		expectedErr := errors.New("close error")
		readCloserMock := mocks.NewMockReadCloser(ctrl)
		readCloserMock.EXPECT().Close().Return(expectedErr)

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", readCloserMock))

		assert.ErrorIs(t, factory.Close(), expectedErr)
	})

	t.Run("should not fail with non-closable resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", &testResource{}))

		assert.NoError(t, factory.Close())
	})

	t.Run("should empty stored resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", &testResource{}))

		assert.NoError(t, factory.Close())

		assert.Empty(t, factory.Stored())
	})
}

func Test_Factory_Available(t *testing.T) {
	t.Run("should return an empty list when there are no entries", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.Empty(t, factory.Available())
	})

	t.Run("should return a sorted list of ids from config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
	})

	t.Run("should return a sorted list of ids of added resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{}).Times(3)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("zulu", &testResource{}))
		require.NoError(t, factory.Store("alpha", &testResource{}))

		assert.Equal(t, []string{"alpha", "zulu"}, factory.Available())
	})

	t.Run("should return a sorted list of ids of combined added resources and config defined resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}}).Times(2)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("charlie", &testResource{}))

		assert.Equal(t, []string{"alpha", "charlie", "zulu"}, factory.Available())
	})
}

func Test_Factory_Stored(t *testing.T) {
	t.Run("should return an empty list of ids if non config as been generated or added", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.Empty(t, factory.Stored())
	})

	t.Run("should return a sorted list of generated resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}}).AnyTimes()

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "zulu"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "zulu"}).Return(&testResource{}, nil)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "alpha"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "alpha"}).Return(&testResource{}, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.GenerateAll())

		assert.Equal(t, []string{"alpha", "zulu"}, factory.Stored())
	})

	t.Run("should return a sorted list of added resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{}).AnyTimes()

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource_1", &testResource{}))
		require.NoError(t, factory.Store("my_resource_2", &testResource{}))

		assert.Equal(t, []string{"my_resource_1", "my_resource_2"}, factory.Stored())
	})

	t.Run("should return a sorted list of a combination of added and generated resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"zulu":  flam.Bag{},
			"alpha": flam.Bag{}}).AnyTimes()

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "zulu"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "zulu"}).Return(&testResource{}, nil)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "alpha"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "alpha"}).Return(&testResource{}, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource_1", &testResource{}))
		require.NoError(t, factory.Store("my_resource_2", &testResource{}))

		require.NoError(t, factory.GenerateAll())

		assert.Equal(t, []string{"alpha", "my_resource_1", "my_resource_2", "zulu"}, factory.Stored())
	})
}

func Test_Factory_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
	factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
		"entry1": flam.Bag{}}).AnyTimes()

	factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
	require.NotNil(t, factory)
	require.NoError(t, e)

	assert.NoError(t, factory.Store("entry2", &testResource{}))

	testCases := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "entry in config",
			id:       "entry1",
			expected: true},
		{
			name:     "manually added entry",
			id:       "entry2",
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
}

func Test_Factory_Get(t *testing.T) {
	t.Run("should return generation error if occurs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Get("nonexistent")
		assert.Nil(t, got)
		assert.ErrorIs(t, e, flam.ErrUnknownResource)
	})

	t.Run("should return the same previously retrieved resource", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		resourceMock := &testResource{}

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(resourceMock, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Get("my_resource")
		require.Same(t, resourceMock, got)
		require.NoError(t, e)

		got, e = factory.Get("my_resource")
		require.Same(t, resourceMock, got)
		require.NoError(t, e)
	})

	t.Run("should return the same previously generated resource", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		resourceMock := &testResource{}

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(resourceMock, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		require.Same(t, resourceMock, got)
		require.NoError(t, e)

		got, e = factory.Get("my_resource")
		require.Same(t, resourceMock, got)
		require.NoError(t, e)
	})
}

func Test_Factory_Store(t *testing.T) {
	t.Run("should return nil reference if resource is nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.ErrorIs(t, factory.Store("my_resource", nil), flam.ErrNilReference)
	})

	t.Run("should return duplicate resource if resource reference exists in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.ErrorIs(t, factory.Store("my_resource", &testResource{}), flam.ErrDuplicateResource)
	})

	t.Run("should return nil error if resource has been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.NoError(t, factory.Store("my_resource", &testResource{}))
	})

	t.Run("should return duplicate resource errir if resource has already been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{}).Times(2)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.NoError(t, factory.Store("my_resource", &testResource{}))
		assert.ErrorIs(t, factory.Store("my_resource", &testResource{}), flam.ErrDuplicateResource)
	})
}

func Test_Factory_Generate(t *testing.T) {
	t.Run("should return unknown resource error for an unknown id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("unknown")
		assert.Nil(t, got)
		assert.ErrorIs(t, e, flam.ErrUnknownResource)
	})

	t.Run("should return unaccepted error if no creator can handle the resource", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.Nil(t, got)
		assert.ErrorIs(t, e, flam.ErrUnacceptedResourceConfig)
	})

	t.Run("should return creator error if occurs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		expectedErr := errors.New("creation failed")
		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(nil, expectedErr)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.Nil(t, got)
		assert.ErrorIs(t, e, expectedErr)
	})

	t.Run("should return the generated resource", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		resourceMock := &testResource{}

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(resourceMock, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.Same(t, resourceMock, got)
		assert.NoError(t, e)
	})

	t.Run("should generate new entries on multiple calls", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}}).Times(2)

		firstResourceMock := &testResource{}
		secondResourceMock := &testResource{}

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		gomock.InOrder(
			creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true),
			creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(firstResourceMock, nil),
			creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true),
			creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(secondResourceMock, nil),
		)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.Same(t, firstResourceMock, got)
		assert.NoError(t, e)

		got, e = factory.Generate("my_resource")
		assert.Same(t, secondResourceMock, got)
		assert.NoError(t, e)
	})

	t.Run("should return the validation error if validator return it", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		expectedErr := errors.New("validation failed")
		validator := func(id string, config flam.Bag) error {
			assert.Equal(t, "my_resource", id)
			assert.Equal(t, flam.Bag{"id": "my_resource"}, config)

			return expectedErr
		}

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, validator, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.Nil(t, got)
		assert.ErrorIs(t, e, expectedErr)
	})

	t.Run("should return the validation error if default driver validator return it", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}})

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, flam.DriverFactoryConfigValidator("resource"), "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.Nil(t, got)
		assert.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
	})

	t.Run("should return generated resource if the driver validator founds a driver field", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{
				"driver": "mock"}})

		resourceMock := &testResource{}

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		gomock.InOrder(
			creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource", "driver": "mock"}).Return(true),
			creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource", "driver": "mock"}).Return(resourceMock, nil),
		)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, flam.DriverFactoryConfigValidator("resource"), "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		got, e := factory.Generate("my_resource")
		assert.NotNil(t, got)
		assert.NoError(t, e)
	})
}

func Test_Factory_GenerateAll(t *testing.T) {
	t.Run("should generate all config entries", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource_1": flam.Bag{},
			"my_resource_2": flam.Bag{}}).AnyTimes()

		firstResourceMock := &testResource{}
		secondResourceMock := &testResource{}

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource_1"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource_1"}).Return(firstResourceMock, nil)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource_2"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource_2"}).Return(secondResourceMock, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.NoError(t, e, factory.GenerateAll())
	})

	t.Run("should return any generation errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}}).Times(2)

		expectedErr := errors.New("generation failed")
		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(nil, expectedErr)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.ErrorIs(t, factory.GenerateAll(), expectedErr)
	})

	t.Run("should not re-generate already stored/generated resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{
			"my_resource": flam.Bag{}}).Times(2)

		creatorMock := mocks.NewMockFactoryResourceCreator[flam.FactoryResource](ctrl)
		creatorMock.EXPECT().Accept(flam.Bag{"id": "my_resource"}).Return(true)
		creatorMock.EXPECT().Create(flam.Bag{"id": "my_resource"}).Return(&testResource{}, nil)
		creators := []flam.FactoryResourceCreator[flam.FactoryResource]{creatorMock}

		factory, e := flam.NewFactory(creators, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		_, e = factory.Generate("my_resource")
		require.NoError(t, e)

		assert.NoError(t, factory.GenerateAll())
	})
}

func Test_Factory_Remove(t *testing.T) {
	t.Run("should return unknown resource if the resource is not stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		assert.ErrorIs(t, factory.Remove("my_resource"), flam.ErrUnknownResource)
	})

	t.Run("should return resource closing errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		expectedErr := errors.New("close failed")
		readCloserMock := mocks.NewMockReadCloser(ctrl)
		readCloserMock.EXPECT().Close().Return(expectedErr)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", readCloserMock))

		assert.ErrorIs(t, factory.Remove("my_resource"), expectedErr)
	})

	t.Run("should remove resource", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{}).Times(2)

		readCloserMock := mocks.NewMockReadCloser(ctrl)
		readCloserMock.EXPECT().Close().Return(nil)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", readCloserMock))

		assert.NoError(t, factory.Remove("my_resource"))

		assert.False(t, factory.Has("my_resource"))
	})
}

func Test_Factory_RemoveAll(t *testing.T) {
	t.Run("should return resource closing errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{})

		expectedErr := errors.New("close failed")
		readCloserMock := mocks.NewMockReadCloser(ctrl)
		readCloserMock.EXPECT().Close().Return(expectedErr)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource", readCloserMock))

		assert.ErrorIs(t, factory.RemoveAll(), expectedErr)
	})

	t.Run("should correctly remove all stored resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factoryConfigMock := mocks.NewMockFactoryConfig(ctrl)
		factoryConfigMock.EXPECT().Get("path").Return(flam.Bag{}).Times(2)

		firstReadCloserMock := mocks.NewMockReadCloser(ctrl)
		firstReadCloserMock.EXPECT().Close().Return(nil)

		secondReadCloserMock := mocks.NewMockReadCloser(ctrl)
		secondReadCloserMock.EXPECT().Close().Return(nil)

		factory, e := flam.NewFactory[flam.FactoryResource](nil, factoryConfigMock, nil, "path")
		require.NotNil(t, factory)
		require.NoError(t, e)

		require.NoError(t, factory.Store("my_resource_1", firstReadCloserMock))
		require.NoError(t, factory.Store("my_resource_2", secondReadCloserMock))

		assert.NoError(t, factory.RemoveAll())
		assert.ElementsMatch(t, []string{}, factory.Stored())
	})
}
