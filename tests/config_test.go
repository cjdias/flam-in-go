package tests

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_Config_Entries(t *testing.T) {
	t.Run("should list entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "value1", "field2": "value2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.ElementsMatch(t, []string{"field1", "field2"}, config.Entries())
		}))
	})

	t.Run("should list entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", "value1"))
			require.NoError(t, config.Set("field2", "value2"))

			assert.ElementsMatch(t, []string{"field1", "field2"}, config.Entries())
		}))
	})

	t.Run("should list combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "value1", "field2": "value2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", "value3"))
			require.NoError(t, config.Set("field4", "value4"))

			assert.ElementsMatch(t, []string{"field1", "field2", "field3", "field4"}, config.Entries())
		}))
	})
}

func Test_Config_Has(t *testing.T) {
	t.Run("should check entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "value1", "field2": "value2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.True(t, config.Has("field1"))
			assert.True(t, config.Has("field2"))
			assert.False(t, config.Has("field3"))
			assert.False(t, config.Has("field4"))
			assert.False(t, config.Has("field5"))
		}))
	})

	t.Run("should check entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", "value1"))
			require.NoError(t, config.Set("field2", "value2"))

			assert.True(t, config.Has("field1"))
			assert.True(t, config.Has("field2"))
			assert.False(t, config.Has("field3"))
			assert.False(t, config.Has("field4"))
			assert.False(t, config.Has("field5"))
		}))
	})

	t.Run("should check combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "value1", "field2": "value2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", "value3"))
			require.NoError(t, config.Set("field4", "value4"))

			assert.True(t, config.Has("field1"))
			assert.True(t, config.Has("field2"))
			assert.True(t, config.Has("field3"))
			assert.True(t, config.Has("field4"))
			assert.False(t, config.Has("field5"))
		}))
	})
}

func Test_Config_Get(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "value1", "field2": "value2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, "value1", config.Get("field1"))
			assert.Equal(t, "value2", config.Get("field2"))
			assert.Nil(t, config.Get("field3"))
			assert.Nil(t, config.Get("field4"))
			assert.Nil(t, config.Get("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", "value1"))
			require.NoError(t, config.Set("field2", "value2"))

			assert.Equal(t, "value1", config.Get("field1"))
			assert.Equal(t, "value2", config.Get("field2"))
			assert.Nil(t, config.Get("field3"))
			assert.Nil(t, config.Get("field4"))
			assert.Nil(t, config.Get("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "value1", "field2": "value2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", "value3"))
			require.NoError(t, config.Set("field4", "value4"))

			assert.Equal(t, "value1", config.Get("field1"))
			assert.Equal(t, "value2", config.Get("field2"))
			assert.Equal(t, "value3", config.Get("field3"))
			assert.Equal(t, "value4", config.Get("field4"))
			assert.Nil(t, config.Get("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []any
			expected any
		}{
			{
				name:     "should return nil on non-existent path",
				data:     flam.Bag{"field1": "value1", "field2": flam.Bag{"field3": "value3"}},
				path:     "invalid",
				expected: nil},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "value1", "field2": flam.Bag{"field3": "value3"}},
				path:     "invalid",
				def:      []any{"default"},
				expected: "default"},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": "value1", "field2": flam.Bag{"field3": "value3"}},
				path:     "field1",
				expected: "value1"},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": "value1", "field2": flam.Bag{"field3": "value3"}},
				path:     "field2.field3",
				expected: "value3"},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Get(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Bool(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": true, "field2": false}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.True(t, config.Bool("field1"))
			assert.False(t, config.Bool("field2"))
			assert.False(t, config.Bool("field3"))
			assert.False(t, config.Bool("field4"))
			assert.False(t, config.Bool("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", true))
			require.NoError(t, config.Set("field2", false))

			assert.True(t, config.Bool("field1"))
			assert.False(t, config.Bool("field2"))
			assert.False(t, config.Bool("field3"))
			assert.False(t, config.Bool("field4"))
			assert.False(t, config.Bool("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": true, "field2": false}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", true))
			require.NoError(t, config.Set("field4", false))

			assert.True(t, config.Bool("field1"))
			assert.False(t, config.Bool("field2"))
			assert.True(t, config.Bool("field3"))
			assert.False(t, config.Bool("field4"))
			assert.False(t, config.Bool("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []bool
			expected bool
		}{
			{
				name:     "should return false on non-existent path",
				data:     flam.Bag{"field1": true, "field2": flam.Bag{"field3": true}},
				path:     "invalid",
				expected: false},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": false, "field2": flam.Bag{"field3": false}},
				path:     "invalid",
				def:      []bool{true},
				expected: true},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": true, "field2": flam.Bag{"field3": true}},
				path:     "field1",
				expected: true},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": true, "field2": flam.Bag{"field3": true}},
				path:     "field2.field3",
				expected: true},
			{
				name:     "should return false on existing path that hold a non-boolean value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": true}},
				path:     "field1",
				expected: false},
			{
				name:     "should return default on existing path that hold a non-boolean value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": true}},
				path:     "field1",
				def:      []bool{true},
				expected: true},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Bool(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Int(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": 1, "field2": 2}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, 1, config.Int("field1"))
			assert.Equal(t, 2, config.Int("field2"))
			assert.Equal(t, 0, config.Int("field3"))
			assert.Equal(t, 0, config.Int("field4"))
			assert.Equal(t, 0, config.Int("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", 1))
			require.NoError(t, config.Set("field2", 2))

			assert.Equal(t, 1, config.Int("field1"))
			assert.Equal(t, 2, config.Int("field2"))
			assert.Equal(t, 0, config.Int("field3"))
			assert.Equal(t, 0, config.Int("field4"))
			assert.Equal(t, 0, config.Int("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": 1, "field2": 2}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", 3))
			require.NoError(t, config.Set("field4", 4))

			assert.Equal(t, 1, config.Int("field1"))
			assert.Equal(t, 2, config.Int("field2"))
			assert.Equal(t, 3, config.Int("field3"))
			assert.Equal(t, 4, config.Int("field4"))
			assert.Equal(t, 0, config.Int("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []int
			expected int
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": 1}},
				path:     "invalid",
				expected: 0},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": 0, "field2": flam.Bag{"field3": 0}},
				path:     "invalid",
				def:      []int{1},
				expected: 1},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": 2}},
				path:     "field1",
				expected: 1},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": 2}},
				path:     "field2.field3",
				expected: 2},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": 1}},
				path:     "field1",
				expected: 0},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": 1}},
				path:     "field1",
				def:      []int{1},
				expected: 1},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Int(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Int8(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int8(1), "field2": int8(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, int8(1), config.Int8("field1"))
			assert.Equal(t, int8(2), config.Int8("field2"))
			assert.Equal(t, int8(0), config.Int8("field3"))
			assert.Equal(t, int8(0), config.Int8("field4"))
			assert.Equal(t, int8(0), config.Int8("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", int8(1)))
			require.NoError(t, config.Set("field2", int8(2)))

			assert.Equal(t, int8(1), config.Int8("field1"))
			assert.Equal(t, int8(2), config.Int8("field2"))
			assert.Equal(t, int8(0), config.Int8("field3"))
			assert.Equal(t, int8(0), config.Int8("field4"))
			assert.Equal(t, int8(0), config.Int8("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int8(1), "field2": int8(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", int8(3)))
			require.NoError(t, config.Set("field4", int8(4)))

			assert.Equal(t, int8(1), config.Int8("field1"))
			assert.Equal(t, int8(2), config.Int8("field2"))
			assert.Equal(t, int8(3), config.Int8("field3"))
			assert.Equal(t, int8(4), config.Int8("field4"))
			assert.Equal(t, int8(0), config.Int8("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []int8
			expected int8
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": int8(1), "field2": flam.Bag{"field3": int8(1)}},
				path:     "invalid",
				expected: int8(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": int8(0), "field2": flam.Bag{"field3": int8(0)}},
				path:     "invalid",
				def:      []int8{1},
				expected: int8(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": int8(1), "field2": flam.Bag{"field3": int8(2)}},
				path:     "field1",
				expected: int8(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": int8(1), "field2": flam.Bag{"field3": int8(2)}},
				path:     "field2.field3",
				expected: int8(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int8(1)}},
				path:     "field1",
				expected: int8(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int8(1)}},
				path:     "field1",
				def:      []int8{1},
				expected: int8(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Int8(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Int16(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int16(1), "field2": int16(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, int16(1), config.Int16("field1"))
			assert.Equal(t, int16(2), config.Int16("field2"))
			assert.Equal(t, int16(0), config.Int16("field3"))
			assert.Equal(t, int16(0), config.Int16("field4"))
			assert.Equal(t, int16(0), config.Int16("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", int16(1)))
			require.NoError(t, config.Set("field2", int16(2)))

			assert.Equal(t, int16(1), config.Int16("field1"))
			assert.Equal(t, int16(2), config.Int16("field2"))
			assert.Equal(t, int16(0), config.Int16("field3"))
			assert.Equal(t, int16(0), config.Int16("field4"))
			assert.Equal(t, int16(0), config.Int16("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int16(1), "field2": int16(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", int16(3)))
			require.NoError(t, config.Set("field4", int16(4)))

			assert.Equal(t, int16(1), config.Int16("field1"))
			assert.Equal(t, int16(2), config.Int16("field2"))
			assert.Equal(t, int16(3), config.Int16("field3"))
			assert.Equal(t, int16(4), config.Int16("field4"))
			assert.Equal(t, int16(0), config.Int16("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []int16
			expected int16
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": int16(1), "field2": flam.Bag{"field3": int16(1)}},
				path:     "invalid",
				expected: int16(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": int16(0), "field2": flam.Bag{"field3": int16(0)}},
				path:     "invalid",
				def:      []int16{1},
				expected: int16(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": int16(1), "field2": flam.Bag{"field3": int16(2)}},
				path:     "field1",
				expected: int16(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": int16(1), "field2": flam.Bag{"field3": int16(2)}},
				path:     "field2.field3",
				expected: int16(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int16(1)}},
				path:     "field1",
				expected: int16(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int16(1)}},
				path:     "field1",
				def:      []int16{1},
				expected: int16(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Int16(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Int32(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int32(1), "field2": int32(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, int32(1), config.Int32("field1"))
			assert.Equal(t, int32(2), config.Int32("field2"))
			assert.Equal(t, int32(0), config.Int32("field3"))
			assert.Equal(t, int32(0), config.Int32("field4"))
			assert.Equal(t, int32(0), config.Int32("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", int32(1)))
			require.NoError(t, config.Set("field2", int32(2)))

			assert.Equal(t, int32(1), config.Int32("field1"))
			assert.Equal(t, int32(2), config.Int32("field2"))
			assert.Equal(t, int32(0), config.Int32("field3"))
			assert.Equal(t, int32(0), config.Int32("field4"))
			assert.Equal(t, int32(0), config.Int32("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int32(1), "field2": int32(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", int32(3)))
			require.NoError(t, config.Set("field4", int32(4)))

			assert.Equal(t, int32(1), config.Int32("field1"))
			assert.Equal(t, int32(2), config.Int32("field2"))
			assert.Equal(t, int32(3), config.Int32("field3"))
			assert.Equal(t, int32(4), config.Int32("field4"))
			assert.Equal(t, int32(0), config.Int32("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []int32
			expected int32
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": int32(1), "field2": flam.Bag{"field3": int32(1)}},
				path:     "invalid",
				expected: int32(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": int32(0), "field2": flam.Bag{"field3": int32(0)}},
				path:     "invalid",
				def:      []int32{1},
				expected: int32(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": int32(1), "field2": flam.Bag{"field3": int32(2)}},
				path:     "field1",
				expected: int32(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": int32(1), "field2": flam.Bag{"field3": int32(2)}},
				path:     "field2.field3",
				expected: int32(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int32(1)}},
				path:     "field1",
				expected: int32(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int32(1)}},
				path:     "field1",
				def:      []int32{1},
				expected: int32(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Int32(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Int64(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int64(1), "field2": int64(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, int64(1), config.Int64("field1"))
			assert.Equal(t, int64(2), config.Int64("field2"))
			assert.Equal(t, int64(0), config.Int64("field3"))
			assert.Equal(t, int64(0), config.Int64("field4"))
			assert.Equal(t, int64(0), config.Int64("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", int64(1)))
			require.NoError(t, config.Set("field2", int64(2)))

			assert.Equal(t, int64(1), config.Int64("field1"))
			assert.Equal(t, int64(2), config.Int64("field2"))
			assert.Equal(t, int64(0), config.Int64("field3"))
			assert.Equal(t, int64(0), config.Int64("field4"))
			assert.Equal(t, int64(0), config.Int64("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": int64(1), "field2": int64(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", int64(3)))
			require.NoError(t, config.Set("field4", int64(4)))

			assert.Equal(t, int64(1), config.Int64("field1"))
			assert.Equal(t, int64(2), config.Int64("field2"))
			assert.Equal(t, int64(3), config.Int64("field3"))
			assert.Equal(t, int64(4), config.Int64("field4"))
			assert.Equal(t, int64(0), config.Int64("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []int64
			expected int64
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": int64(1), "field2": flam.Bag{"field3": int64(1)}},
				path:     "invalid",
				expected: int64(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": int64(0), "field2": flam.Bag{"field3": int64(0)}},
				path:     "invalid",
				def:      []int64{1},
				expected: int64(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": int64(1), "field2": flam.Bag{"field3": int64(2)}},
				path:     "field1",
				expected: int64(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": int64(1), "field2": flam.Bag{"field3": int64(2)}},
				path:     "field2.field3",
				expected: int64(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int64(1)}},
				path:     "field1",
				expected: int64(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": int64(1)}},
				path:     "field1",
				def:      []int64{1},
				expected: int64(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Int64(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Uint(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint(1), "field2": uint(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, uint(1), config.Uint("field1"))
			assert.Equal(t, uint(2), config.Uint("field2"))
			assert.Equal(t, uint(0), config.Uint("field3"))
			assert.Equal(t, uint(0), config.Uint("field4"))
			assert.Equal(t, uint(0), config.Uint("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", uint(1)))
			require.NoError(t, config.Set("field2", uint(2)))

			assert.Equal(t, uint(1), config.Uint("field1"))
			assert.Equal(t, uint(2), config.Uint("field2"))
			assert.Equal(t, uint(0), config.Uint("field3"))
			assert.Equal(t, uint(0), config.Uint("field4"))
			assert.Equal(t, uint(0), config.Uint("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint(1), "field2": uint(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", uint(3)))
			require.NoError(t, config.Set("field4", uint(4)))

			assert.Equal(t, uint(1), config.Uint("field1"))
			assert.Equal(t, uint(2), config.Uint("field2"))
			assert.Equal(t, uint(3), config.Uint("field3"))
			assert.Equal(t, uint(4), config.Uint("field4"))
			assert.Equal(t, uint(0), config.Uint("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []uint
			expected uint
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": uint(1), "field2": flam.Bag{"field3": uint(1)}},
				path:     "invalid",
				expected: uint(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": uint(0), "field2": flam.Bag{"field3": uint(0)}},
				path:     "invalid",
				def:      []uint{1},
				expected: uint(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": uint(1), "field2": flam.Bag{"field3": uint(2)}},
				path:     "field1",
				expected: uint(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": uint(1), "field2": flam.Bag{"field3": uint(2)}},
				path:     "field2.field3",
				expected: uint(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint(1)}},
				path:     "field1",
				expected: uint(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint(1)}},
				path:     "field1",
				def:      []uint{1},
				expected: uint(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Uint(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Uint8(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint8(1), "field2": uint8(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, uint8(1), config.Uint8("field1"))
			assert.Equal(t, uint8(2), config.Uint8("field2"))
			assert.Equal(t, uint8(0), config.Uint8("field3"))
			assert.Equal(t, uint8(0), config.Uint8("field4"))
			assert.Equal(t, uint8(0), config.Uint8("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", uint8(1)))
			require.NoError(t, config.Set("field2", uint8(2)))

			assert.Equal(t, uint8(1), config.Uint8("field1"))
			assert.Equal(t, uint8(2), config.Uint8("field2"))
			assert.Equal(t, uint8(0), config.Uint8("field3"))
			assert.Equal(t, uint8(0), config.Uint8("field4"))
			assert.Equal(t, uint8(0), config.Uint8("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint8(1), "field2": uint8(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", uint8(3)))
			require.NoError(t, config.Set("field4", uint8(4)))

			assert.Equal(t, uint8(1), config.Uint8("field1"))
			assert.Equal(t, uint8(2), config.Uint8("field2"))
			assert.Equal(t, uint8(3), config.Uint8("field3"))
			assert.Equal(t, uint8(4), config.Uint8("field4"))
			assert.Equal(t, uint8(0), config.Uint8("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []uint8
			expected uint8
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": uint8(1), "field2": flam.Bag{"field3": uint8(1)}},
				path:     "invalid",
				expected: uint8(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": uint8(0), "field2": flam.Bag{"field3": uint8(0)}},
				path:     "invalid",
				def:      []uint8{1},
				expected: uint8(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": uint8(1), "field2": flam.Bag{"field3": uint8(2)}},
				path:     "field1",
				expected: uint8(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": uint8(1), "field2": flam.Bag{"field3": uint8(2)}},
				path:     "field2.field3",
				expected: uint8(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint8(1)}},
				path:     "field1",
				expected: uint8(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint8(1)}},
				path:     "field1",
				def:      []uint8{1},
				expected: uint8(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Uint8(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Uint16(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint16(1), "field2": uint16(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, uint16(1), config.Uint16("field1"))
			assert.Equal(t, uint16(2), config.Uint16("field2"))
			assert.Equal(t, uint16(0), config.Uint16("field3"))
			assert.Equal(t, uint16(0), config.Uint16("field4"))
			assert.Equal(t, uint16(0), config.Uint16("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", uint16(1)))
			require.NoError(t, config.Set("field2", uint16(2)))

			assert.Equal(t, uint16(1), config.Uint16("field1"))
			assert.Equal(t, uint16(2), config.Uint16("field2"))
			assert.Equal(t, uint16(0), config.Uint16("field3"))
			assert.Equal(t, uint16(0), config.Uint16("field4"))
			assert.Equal(t, uint16(0), config.Uint16("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint16(1), "field2": uint16(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", uint16(3)))
			require.NoError(t, config.Set("field4", uint16(4)))

			assert.Equal(t, uint16(1), config.Uint16("field1"))
			assert.Equal(t, uint16(2), config.Uint16("field2"))
			assert.Equal(t, uint16(3), config.Uint16("field3"))
			assert.Equal(t, uint16(4), config.Uint16("field4"))
			assert.Equal(t, uint16(0), config.Uint16("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []uint16
			expected uint16
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": uint16(1), "field2": flam.Bag{"field3": uint16(1)}},
				path:     "invalid",
				expected: uint16(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": uint16(0), "field2": flam.Bag{"field3": uint16(0)}},
				path:     "invalid",
				def:      []uint16{1},
				expected: uint16(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": uint16(1), "field2": flam.Bag{"field3": uint16(2)}},
				path:     "field1",
				expected: uint16(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": uint16(1), "field2": flam.Bag{"field3": uint16(2)}},
				path:     "field2.field3",
				expected: uint16(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint16(1)}},
				path:     "field1",
				expected: uint16(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint16(1)}},
				path:     "field1",
				def:      []uint16{1},
				expected: uint16(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Uint16(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Uint32(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint32(1), "field2": uint32(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, uint32(1), config.Uint32("field1"))
			assert.Equal(t, uint32(2), config.Uint32("field2"))
			assert.Equal(t, uint32(0), config.Uint32("field3"))
			assert.Equal(t, uint32(0), config.Uint32("field4"))
			assert.Equal(t, uint32(0), config.Uint32("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", uint32(1)))
			require.NoError(t, config.Set("field2", uint32(2)))

			assert.Equal(t, uint32(1), config.Uint32("field1"))
			assert.Equal(t, uint32(2), config.Uint32("field2"))
			assert.Equal(t, uint32(0), config.Uint32("field3"))
			assert.Equal(t, uint32(0), config.Uint32("field4"))
			assert.Equal(t, uint32(0), config.Uint32("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint32(1), "field2": uint32(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", uint32(3)))
			require.NoError(t, config.Set("field4", uint32(4)))

			assert.Equal(t, uint32(1), config.Uint32("field1"))
			assert.Equal(t, uint32(2), config.Uint32("field2"))
			assert.Equal(t, uint32(3), config.Uint32("field3"))
			assert.Equal(t, uint32(4), config.Uint32("field4"))
			assert.Equal(t, uint32(0), config.Uint32("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []uint32
			expected uint32
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": uint32(1), "field2": flam.Bag{"field3": uint32(1)}},
				path:     "invalid",
				expected: uint32(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": uint32(0), "field2": flam.Bag{"field3": uint32(0)}},
				path:     "invalid",
				def:      []uint32{1},
				expected: uint32(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": uint32(1), "field2": flam.Bag{"field3": uint32(2)}},
				path:     "field1",
				expected: uint32(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": uint32(1), "field2": flam.Bag{"field3": uint32(2)}},
				path:     "field2.field3",
				expected: uint32(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint32(1)}},
				path:     "field1",
				expected: uint32(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint32(1)}},
				path:     "field1",
				def:      []uint32{1},
				expected: uint32(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Uint32(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Uint64(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint64(1), "field2": uint64(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, uint64(1), config.Uint64("field1"))
			assert.Equal(t, uint64(2), config.Uint64("field2"))
			assert.Equal(t, uint64(0), config.Uint64("field3"))
			assert.Equal(t, uint64(0), config.Uint64("field4"))
			assert.Equal(t, uint64(0), config.Uint64("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", uint64(1)))
			require.NoError(t, config.Set("field2", uint64(2)))

			assert.Equal(t, uint64(1), config.Uint64("field1"))
			assert.Equal(t, uint64(2), config.Uint64("field2"))
			assert.Equal(t, uint64(0), config.Uint64("field3"))
			assert.Equal(t, uint64(0), config.Uint64("field4"))
			assert.Equal(t, uint64(0), config.Uint64("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": uint64(1), "field2": uint64(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", uint64(3)))
			require.NoError(t, config.Set("field4", uint64(4)))

			assert.Equal(t, uint64(1), config.Uint64("field1"))
			assert.Equal(t, uint64(2), config.Uint64("field2"))
			assert.Equal(t, uint64(3), config.Uint64("field3"))
			assert.Equal(t, uint64(4), config.Uint64("field4"))
			assert.Equal(t, uint64(0), config.Uint64("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []uint64
			expected uint64
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": uint64(1), "field2": flam.Bag{"field3": uint64(1)}},
				path:     "invalid",
				expected: uint64(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": uint64(0), "field2": flam.Bag{"field3": uint64(0)}},
				path:     "invalid",
				def:      []uint64{1},
				expected: uint64(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": uint64(1), "field2": flam.Bag{"field3": uint64(2)}},
				path:     "field1",
				expected: uint64(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": uint64(1), "field2": flam.Bag{"field3": uint64(2)}},
				path:     "field2.field3",
				expected: uint64(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint64(1)}},
				path:     "field1",
				expected: uint64(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": uint64(1)}},
				path:     "field1",
				def:      []uint64{1},
				expected: uint64(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Uint64(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Float32(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": float32(1), "field2": float32(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, float32(1), config.Float32("field1"))
			assert.Equal(t, float32(2), config.Float32("field2"))
			assert.Equal(t, float32(0), config.Float32("field3"))
			assert.Equal(t, float32(0), config.Float32("field4"))
			assert.Equal(t, float32(0), config.Float32("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", float32(1)))
			require.NoError(t, config.Set("field2", float32(2)))

			assert.Equal(t, float32(1), config.Float32("field1"))
			assert.Equal(t, float32(2), config.Float32("field2"))
			assert.Equal(t, float32(0), config.Float32("field3"))
			assert.Equal(t, float32(0), config.Float32("field4"))
			assert.Equal(t, float32(0), config.Float32("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": float32(1), "field2": float32(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", float32(3)))
			require.NoError(t, config.Set("field4", float32(4)))

			assert.Equal(t, float32(1), config.Float32("field1"))
			assert.Equal(t, float32(2), config.Float32("field2"))
			assert.Equal(t, float32(3), config.Float32("field3"))
			assert.Equal(t, float32(4), config.Float32("field4"))
			assert.Equal(t, float32(0), config.Float32("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []float32
			expected float32
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": float32(1), "field2": flam.Bag{"field3": float32(1)}},
				path:     "invalid",
				expected: float32(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": float32(0), "field2": flam.Bag{"field3": float32(0)}},
				path:     "invalid",
				def:      []float32{1},
				expected: float32(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": float32(1), "field2": flam.Bag{"field3": float32(2)}},
				path:     "field1",
				expected: float32(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": float32(1), "field2": flam.Bag{"field3": float32(2)}},
				path:     "field2.field3",
				expected: float32(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": float32(1)}},
				path:     "field1",
				expected: float32(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": float32(1)}},
				path:     "field1",
				def:      []float32{1},
				expected: float32(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Float32(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Float64(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": float64(1), "field2": float64(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, float64(1), config.Float64("field1"))
			assert.Equal(t, float64(2), config.Float64("field2"))
			assert.Equal(t, float64(0), config.Float64("field3"))
			assert.Equal(t, float64(0), config.Float64("field4"))
			assert.Equal(t, float64(0), config.Float64("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", float64(1)))
			require.NoError(t, config.Set("field2", float64(2)))

			assert.Equal(t, float64(1), config.Float64("field1"))
			assert.Equal(t, float64(2), config.Float64("field2"))
			assert.Equal(t, float64(0), config.Float64("field3"))
			assert.Equal(t, float64(0), config.Float64("field4"))
			assert.Equal(t, float64(0), config.Float64("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": float64(1), "field2": float64(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", float64(3)))
			require.NoError(t, config.Set("field4", float64(4)))

			assert.Equal(t, float64(1), config.Float64("field1"))
			assert.Equal(t, float64(2), config.Float64("field2"))
			assert.Equal(t, float64(3), config.Float64("field3"))
			assert.Equal(t, float64(4), config.Float64("field4"))
			assert.Equal(t, float64(0), config.Float64("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []float64
			expected float64
		}{
			{
				name:     "should return 0 on non-existent path",
				data:     flam.Bag{"field1": float64(1), "field2": flam.Bag{"field3": float64(1)}},
				path:     "invalid",
				expected: float64(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": float64(0), "field2": flam.Bag{"field3": float64(0)}},
				path:     "invalid",
				def:      []float64{1},
				expected: float64(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": float64(1), "field2": flam.Bag{"field3": float64(2)}},
				path:     "field1",
				expected: float64(1)},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": float64(1), "field2": flam.Bag{"field3": float64(2)}},
				path:     "field2.field3",
				expected: float64(2)},
			{
				name:     "should return 0 on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": float64(1)}},
				path:     "field1",
				expected: float64(0)},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": float64(1)}},
				path:     "field1",
				def:      []float64{1},
				expected: float64(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Float64(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_String(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "string1", "field2": "string2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, "string1", config.String("field1"))
			assert.Equal(t, "string2", config.String("field2"))
			assert.Equal(t, "", config.String("field3"))
			assert.Equal(t, "", config.String("field4"))
			assert.Equal(t, "", config.String("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", "string1"))
			require.NoError(t, config.Set("field2", "string2"))

			assert.Equal(t, "string1", config.String("field1"))
			assert.Equal(t, "string2", config.String("field2"))
			assert.Equal(t, "", config.String("field3"))
			assert.Equal(t, "", config.String("field4"))
			assert.Equal(t, "", config.String("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": "string1", "field2": "string2"}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", "string3"))
			require.NoError(t, config.Set("field4", "string4"))

			assert.Equal(t, "string1", config.String("field1"))
			assert.Equal(t, "string2", config.String("field2"))
			assert.Equal(t, "string3", config.String("field3"))
			assert.Equal(t, "string4", config.String("field4"))
			assert.Equal(t, "", config.String("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []string
			expected string
		}{
			{
				name:     "should return empty string on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: ""},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      []string{"1"},
				expected: "1"},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": "1", "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: "1"},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": "1", "field2": flam.Bag{"field3": "2"}},
				path:     "field2.field3",
				expected: "2"},
			{
				name:     "should return empty string on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: ""},
			{
				name:     "should return default on existing path that hold a non-integer value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      []string{"1"},
				expected: "1"},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.String(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_StringMap(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{
			"field1": map[string]any{"field1": "value1"},
			"field2": map[string]any{"field2": "value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, map[string]any{"field1": "value1"}, config.StringMap("field1"))
			assert.Equal(t, map[string]any{"field2": "value2"}, config.StringMap("field2"))
			assert.Nil(t, config.StringMap("field3"))
			assert.Nil(t, config.StringMap("field4"))
			assert.Nil(t, config.StringMap("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", map[string]any{"field1": "value1"}))
			require.NoError(t, config.Set("field2", map[string]any{"field2": "value2"}))

			assert.Equal(t, map[string]any{"field1": "value1"}, config.StringMap("field1"))
			assert.Equal(t, map[string]any{"field2": "value2"}, config.StringMap("field2"))
			assert.Nil(t, config.StringMap("field3"))
			assert.Nil(t, config.StringMap("field4"))
			assert.Nil(t, config.StringMap("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{
			"field1": map[string]any{"field1": "value1"},
			"field2": map[string]any{"field2": "value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", map[string]any{"field3": "value3"}))
			require.NoError(t, config.Set("field4", map[string]any{"field4": "value4"}))

			assert.Equal(t, map[string]any{"field1": "value1"}, config.StringMap("field1"))
			assert.Equal(t, map[string]any{"field2": "value2"}, config.StringMap("field2"))
			assert.Equal(t, map[string]any{"field3": "value3"}, config.StringMap("field3"))
			assert.Equal(t, map[string]any{"field4": "value4"}, config.StringMap("field4"))
			assert.Nil(t, config.StringMap("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []map[string]any
			expected map[string]any
		}{
			{
				name:     "should return empty string map on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: map[string]any(nil)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      []map[string]any{{"1": "1"}},
				expected: map[string]any{"1": "1"}},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": map[string]any{"1": "1"}, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: map[string]any{"1": "1"}},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": map[string]any{"1": "1"}, "field2": flam.Bag{"field3": map[string]any{"2": "2"}}},
				path:     "field2.field3",
				expected: map[string]any{"2": "2"}},
			{
				name:     "should return empty string map on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: map[string]any(nil)},
			{
				name:     "should return default on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      []map[string]any{{"1": "1"}},
				expected: map[string]any{"1": "1"}},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.StringMap(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_StringMapString(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{
			"field1": map[string]string{"field1": "value1"},
			"field2": map[string]string{"field2": "value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, map[string]string{"field1": "value1"}, config.StringMapString("field1"))
			assert.Equal(t, map[string]string{"field2": "value2"}, config.StringMapString("field2"))
			assert.Nil(t, config.StringMapString("field3"))
			assert.Nil(t, config.StringMapString("field4"))
			assert.Nil(t, config.StringMapString("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", map[string]string{"field1": "value1"}))
			require.NoError(t, config.Set("field2", map[string]string{"field2": "value2"}))

			assert.Equal(t, map[string]string{"field1": "value1"}, config.StringMapString("field1"))
			assert.Equal(t, map[string]string{"field2": "value2"}, config.StringMapString("field2"))
			assert.Nil(t, config.StringMapString("field3"))
			assert.Nil(t, config.StringMapString("field4"))
			assert.Nil(t, config.StringMapString("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{
			"field1": map[string]string{"field1": "value1"},
			"field2": map[string]string{"field2": "value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", map[string]string{"field3": "value3"}))
			require.NoError(t, config.Set("field4", map[string]string{"field4": "value4"}))

			assert.Equal(t, map[string]string{"field1": "value1"}, config.StringMapString("field1"))
			assert.Equal(t, map[string]string{"field2": "value2"}, config.StringMapString("field2"))
			assert.Equal(t, map[string]string{"field3": "value3"}, config.StringMapString("field3"))
			assert.Equal(t, map[string]string{"field4": "value4"}, config.StringMapString("field4"))
			assert.Nil(t, config.StringMapString("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []map[string]string
			expected map[string]string
		}{
			{
				name:     "should return empty string map on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: map[string]string(nil)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      []map[string]string{{"1": "1"}},
				expected: map[string]string{"1": "1"}},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": map[string]string{"1": "1"}, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: map[string]string{"1": "1"}},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": map[string]string{"1": "1"}, "field2": flam.Bag{"field3": map[string]string{"2": "2"}}},
				path:     "field2.field3",
				expected: map[string]string{"2": "2"}},
			{
				name:     "should return empty string map on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: map[string]string(nil)},
			{
				name:     "should return default on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      []map[string]string{{"1": "1"}},
				expected: map[string]string{"1": "1"}},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.StringMapString(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Slice(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": []any{"value1"}, "field2": []any{"value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, []any{"value1"}, config.Slice("field1"))
			assert.Equal(t, []any{"value2"}, config.Slice("field2"))
			assert.Nil(t, config.Slice("field3"))
			assert.Nil(t, config.Slice("field4"))
			assert.Nil(t, config.Slice("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", []any{"value1"}))
			require.NoError(t, config.Set("field2", []any{"value2"}))

			assert.Equal(t, []any{"value1"}, config.Slice("field1"))
			assert.Equal(t, []any{"value2"}, config.Slice("field2"))
			assert.Nil(t, config.Slice("field3"))
			assert.Nil(t, config.Slice("field4"))
			assert.Nil(t, config.Slice("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": []any{"value1"}, "field2": []any{"value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", []any{"value3"}))
			require.NoError(t, config.Set("field4", []any{"value4"}))

			assert.Equal(t, []any{"value1"}, config.Slice("field1"))
			assert.Equal(t, []any{"value2"}, config.Slice("field2"))
			assert.Equal(t, []any{"value3"}, config.Slice("field3"))
			assert.Equal(t, []any{"value4"}, config.Slice("field4"))
			assert.Nil(t, config.Slice("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      [][]any
			expected []any
		}{
			{
				name:     "should return empty slice on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: []any(nil)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      [][]any{{"1"}},
				expected: []any{"1"}},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": []any{"1"}, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: []any{"1"}},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": []any{"1"}, "field2": flam.Bag{"field3": []any{"2"}}},
				path:     "field2.field3",
				expected: []any{"2"}},
			{
				name:     "should return empty slice on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: []any(nil)},
			{
				name:     "should return default on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      [][]any{{"1"}},
				expected: []any{"1"}},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Slice(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_StringSlice(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": []string{"value1"}, "field2": []string{"value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, []string{"value1"}, config.StringSlice("field1"))
			assert.Equal(t, []string{"value2"}, config.StringSlice("field2"))
			assert.Nil(t, config.StringSlice("field3"))
			assert.Nil(t, config.StringSlice("field4"))
			assert.Nil(t, config.StringSlice("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", []string{"value1"}))
			require.NoError(t, config.Set("field2", []string{"value2"}))

			assert.Equal(t, []string{"value1"}, config.StringSlice("field1"))
			assert.Equal(t, []string{"value2"}, config.StringSlice("field2"))
			assert.Nil(t, config.StringSlice("field3"))
			assert.Nil(t, config.StringSlice("field4"))
			assert.Nil(t, config.StringSlice("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": []string{"value1"}, "field2": []string{"value2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", []string{"value3"}))
			require.NoError(t, config.Set("field4", []string{"value4"}))

			assert.Equal(t, []string{"value1"}, config.StringSlice("field1"))
			assert.Equal(t, []string{"value2"}, config.StringSlice("field2"))
			assert.Equal(t, []string{"value3"}, config.StringSlice("field3"))
			assert.Equal(t, []string{"value4"}, config.StringSlice("field4"))
			assert.Nil(t, config.StringSlice("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      [][]string
			expected []string
		}{
			{
				name:     "should return empty slice on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: []string(nil)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      [][]string{{"1"}},
				expected: []string{"1"}},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": []string{"1"}, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: []string{"1"}},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": []string{"1"}, "field2": flam.Bag{"field3": []string{"2"}}},
				path:     "field2.field3",
				expected: []string{"2"}},
			{
				name:     "should return empty slice on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: []string(nil)},
			{
				name:     "should return default on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      [][]string{{"1"}},
				expected: []string{"1"}},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.StringSlice(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Duration(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": time.Duration(1), "field2": time.Duration(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, time.Duration(1), config.Duration("field1"))
			assert.Equal(t, time.Duration(2), config.Duration("field2"))
			assert.Equal(t, time.Duration(0), config.Duration("field3"))
			assert.Equal(t, time.Duration(0), config.Duration("field4"))
			assert.Equal(t, time.Duration(0), config.Duration("field5"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", time.Duration(1)))
			require.NoError(t, config.Set("field2", time.Duration(2)))

			assert.Equal(t, time.Duration(1), config.Duration("field1"))
			assert.Equal(t, time.Duration(2), config.Duration("field2"))
			assert.Equal(t, time.Duration(0), config.Duration("field3"))
			assert.Equal(t, time.Duration(0), config.Duration("field4"))
			assert.Equal(t, time.Duration(0), config.Duration("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": time.Duration(1), "field2": time.Duration(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", time.Duration(3)))
			require.NoError(t, config.Set("field4", time.Duration(4)))

			assert.Equal(t, time.Duration(1), config.Duration("field1"))
			assert.Equal(t, time.Duration(2), config.Duration("field2"))
			assert.Equal(t, time.Duration(3), config.Duration("field3"))
			assert.Equal(t, time.Duration(4), config.Duration("field4"))
			assert.Equal(t, time.Duration(0), config.Duration("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []time.Duration
			expected time.Duration
		}{
			{
				name:     "should return zero duration on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: time.Duration(0)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      []time.Duration{1},
				expected: time.Duration(1)},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": time.Duration(1), "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: time.Duration(1)},
			{
				name:     "should return value on existent path (int conversion)",
				data:     flam.Bag{"field1": 1000, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: time.Duration(1000) * time.Millisecond},
			{
				name:     "should return value on existent path (int64 conversion)",
				data:     flam.Bag{"field1": int64(1000), "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: time.Duration(1000) * time.Millisecond},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": time.Duration(1), "field2": flam.Bag{"field3": time.Duration(2)}},
				path:     "field2.field3",
				expected: time.Duration(2)},
			{
				name:     "should return zero duration on existing path that hold a non-duration value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: time.Duration(0)},
			{
				name:     "should return default on existing path that hold a non-duration value",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      []time.Duration{1},
				expected: time.Duration(1)},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Duration(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Bag(t *testing.T) {
	t.Run("should retrieve entries from the loaded sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{
			"field1": flam.Bag{"field1": "string1"},
			"field2": flam.Bag{"field2": "string2"}}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))

			assert.Equal(t, flam.Bag{"field1": "string1"}, config.Bag("field1"))
			assert.Equal(t, flam.Bag{"field2": "string2"}, config.Bag("field2"))
			assert.Nil(t, config.Bag("field3"))
			assert.Nil(t, config.Bag("field3"))
			assert.Nil(t, config.Bag("field3"))
		}))
	})

	t.Run("should retrieve entries stored directly", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.Set("field1", flam.Bag{"field1": "string1"}))
			require.NoError(t, config.Set("field2", flam.Bag{"field2": "string2"}))

			assert.Equal(t, flam.Bag{"field1": "string1"}, config.Bag("field1"))
			assert.Equal(t, flam.Bag{"field2": "string2"}, config.Bag("field2"))
			assert.Nil(t, config.Bag("field3"))
			assert.Nil(t, config.Bag("field4"))
			assert.Nil(t, config.Bag("field5"))
		}))
	})

	t.Run("should retrieve combined entries from the loaded sources and stored directly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		data := flam.Bag{"field1": time.Duration(1), "field2": time.Duration(2)}
		configSourceMock := mocks.NewMockConfigSource(ctrl)
		configSourceMock.EXPECT().Get("", flam.Bag{}).Return(data)
		configSourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("my_source", configSourceMock))
			require.NoError(t, config.Set("field3", time.Duration(3)))
			require.NoError(t, config.Set("field4", time.Duration(4)))

			assert.Equal(t, time.Duration(1), config.Duration("field1"))
			assert.Equal(t, time.Duration(2), config.Duration("field2"))
			assert.Equal(t, time.Duration(3), config.Duration("field3"))
			assert.Equal(t, time.Duration(4), config.Duration("field4"))
			assert.Equal(t, time.Duration(0), config.Duration("field5"))
		}))
	})

	t.Run("aggregation bag access", func(t *testing.T) {
		scenarios := []struct {
			name     string
			data     flam.Bag
			path     string
			def      []flam.Bag
			expected flam.Bag
		}{
			{
				name:     "should return empty bag on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				expected: flam.Bag(nil)},
			{
				name:     "should return passed default on non-existent path",
				data:     flam.Bag{"field1": "string", "field2": flam.Bag{"field3": "string"}},
				path:     "invalid",
				def:      []flam.Bag{{"1": "1"}},
				expected: flam.Bag{"1": "1"}},
			{
				name:     "should return value on existent path",
				data:     flam.Bag{"field1": flam.Bag{"1": "1"}, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: flam.Bag{"1": "1"}},
			{
				name:     "should return value on existing inner path",
				data:     flam.Bag{"field1": flam.Bag{"1": "1"}, "field2": flam.Bag{"field3": flam.Bag{"2": "2"}}},
				path:     "field2.field3",
				expected: flam.Bag{"2": "2"}},
			{
				name:     "should return empty bag on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				expected: flam.Bag(nil)},
			{
				name:     "should return default on existing path that hold a non-string-map value",
				data:     flam.Bag{"field1": 1, "field2": flam.Bag{"field3": "2"}},
				path:     "field1",
				def:      []flam.Bag{{"1": "1"}},
				expected: flam.Bag{"1": "1"}},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.data)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					assert.Equal(t, scenario.expected, config.Bag(scenario.path, scenario.def...))
				}))
			})
		}
	})
}

func Test_Config_Set(t *testing.T) {
	t.Run("should return error if is to store at root", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.ErrorIs(t, config.Set("", "value"), flam.ErrBagInvalidPath)
		}))
	})

	t.Run("should correctly store the value", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.NoError(t, config.Set("field.subfield", "value"))

			assert.Equal(t, flam.Bag{"subfield": "value"}, config.Bag("field"))
			assert.Equal(t, "value", config.String("field.subfield"))
		}))
	})

	t.Run("should override any source value", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		sourceMock := mocks.NewMockConfigSource(ctrl)
		sourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{
			"field": flam.Bag{
				"subfield": "value"}})
		sourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, factory.Store("source", sourceMock))

			assert.NoError(t, config.Set("field.subfield", "value2"))

			assert.Equal(t, flam.Bag{"subfield": "value2"}, config.Bag("field"))
			assert.Equal(t, "value2", config.String("field.subfield"))
		}))
	})
}

func Test_Config_Populate(t *testing.T) {
	type simpleStruct struct {
		Field int
	}

	type complexStruct struct {
		Name     string `mapstructure:"name"`
		Value    int    `mapstructure:"value"`
		Nested   simpleStruct
		Children []string
	}

	t.Run("without path", func(t *testing.T) {
		scenarios := []struct {
			test        string
			bag         flam.Bag
			target      any
			expected    any
			expectedErr error
		}{
			{
				test:     "should populate a struct with a simple scalar value",
				bag:      flam.Bag{"field": 123},
				target:   &simpleStruct{},
				expected: &simpleStruct{Field: 123}},
			{
				test: "should populate a complex struct with tags",
				bag: flam.Bag{
					"name":  "test_name",
					"value": 999,
					"Nested": flam.Bag{
						"Field": 789},
					"Children": []any{"child1", "child2"}},
				target: &complexStruct{},
				expected: &complexStruct{
					Name:     "test_name",
					Value:    999,
					Nested:   simpleStruct{Field: 789},
					Children: []string{"child1", "child2"}},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.test, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.bag)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					e := config.Populate(scenario.target)

					if scenario.expectedErr != nil {
						assert.ErrorIs(t, e, scenario.expectedErr)
						return
					}

					assert.NoError(t, e)
					assert.Equal(t, scenario.expected, scenario.target)
				}))
			})
		}
	})

	t.Run("with path", func(t *testing.T) {
		scenarios := []struct {
			test        string
			bag         flam.Bag
			path        string
			target      any
			expected    any
			expectedErr error
		}{
			{
				test:     "should populate a struct from a nested path",
				bag:      flam.Bag{"config": flam.Bag{"field": 456}},
				path:     "config",
				target:   &simpleStruct{},
				expected: &simpleStruct{Field: 456}},
			{
				test:        "should return an error for an invalid path",
				bag:         flam.Bag{"config": flam.Bag{"field": 456}},
				path:        "invalid.path",
				target:      &simpleStruct{},
				expectedErr: flam.ErrBagInvalidPath},
			{
				test: "should populate a complex struct from a nested path",
				bag: flam.Bag{
					"data": flam.Bag{
						"name":  "nested_name",
						"value": 111,
						"Nested": flam.Bag{
							"Field": 222},
						"Children": []any{"c1"}}},
				path:   "data",
				target: &complexStruct{},
				expected: &complexStruct{
					Name:     "nested_name",
					Value:    111,
					Nested:   simpleStruct{Field: 222},
					Children: []string{"c1"},
				},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.test, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				app := flam.NewApplication()
				defer func() { _ = app.Close() }()

				sourceMock := mocks.NewMockConfigSource(ctrl)
				sourceMock.EXPECT().Get("", flam.Bag{}).Return(scenario.bag)
				sourceMock.EXPECT().Close().Return(nil)

				assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
					require.NoError(t, factory.Store("source", sourceMock))

					e := config.Populate(scenario.target, scenario.path)

					if scenario.expectedErr != nil {
						assert.ErrorIs(t, e, scenario.expectedErr)
						return
					}

					assert.NoError(t, e)
					assert.Equal(t, scenario.expected, scenario.target)
				}))
			})
		}
	})
}

func Test_Config_HasObserver(t *testing.T) {
	t.Run("should return false if the observer is not present", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		observer := flam.ConfigObserver(func(old any, new any) {})

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.AddObserver("id", "field", observer))

			assert.False(t, config.HasObserver("other", "field"))
			assert.False(t, config.HasObserver("id", "other"))
		}))
	})

	t.Run("should return true if the observer is present", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		observer := flam.ConfigObserver(func(old any, new any) {})

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.AddObserver("id", "field", observer))
			assert.True(t, config.HasObserver("id", "field"))
		}))
	})
}

func Test_Config_AddObserver(t *testing.T) {
	t.Run("should return error on nil callback", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			assert.ErrorIs(t, config.AddObserver("", "", nil), flam.ErrNilReference)
		}))
	})

	t.Run("should store observer and be called on value change", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		observer := flam.ConfigObserver(func(old any, new any) {})

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.AddObserver("id", "field", observer))
			assert.ErrorIs(t, config.AddObserver("id", "field", observer), flam.ErrDuplicateConfigObserver)
		}))
	})

	t.Run("should store observer and be called on value change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		called := false
		observer := flam.ConfigObserver(func(old any, new any) {
			assert.Nil(t, old)
			assert.Equal(t, "value1", new)
			called = true
		})

		sourceMock := mocks.NewMockConfigSource(ctrl)
		sourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{"field": "value1"})
		sourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, config.AddObserver("id", "field", observer))

			require.NoError(t, factory.Store("source", sourceMock))
			assert.True(t, called)
			assert.Equal(t, "value1", config.Get("field"))
		}))
	})
}

func Test_Config_RemoveObserver(t *testing.T) {
	t.Run("should no error if observer does not exist", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		assert.NoError(t, app.Container().Invoke(func(config flam.Config) {
			require.NoError(t, config.RemoveObserver("id"))
		}))
	})

	t.Run("should remove observer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		called := false
		observer := flam.ConfigObserver(func(old any, new any) {
			called = true
		})

		sourceMock := mocks.NewMockConfigSource(ctrl)
		sourceMock.EXPECT().Get("", flam.Bag{}).Return(flam.Bag{"field": "value1"})
		sourceMock.EXPECT().Close().Return(nil)

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory, config flam.Config) {
			require.NoError(t, config.AddObserver("id", "field", observer))
			require.NoError(t, config.RemoveObserver("id"))

			require.NoError(t, factory.Store("source", sourceMock))

			assert.False(t, called)
			assert.Equal(t, "value1", config.Get("field"))
		}))
	})
}
