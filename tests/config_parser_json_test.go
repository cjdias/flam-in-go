package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_JsonConfigParser(t *testing.T) {
	t.Run("should return reader error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("my error")
		readerMock := mocks.NewMockReadCloser(ctrl)
		readerMock.EXPECT().Read(gomock.Any()).Return(0, expectedErr)

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			parsed, e := parser.Parse(readerMock)
			assert.Nil(t, parsed)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return parsing error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			parsed, e := parser.Parse(strings.NewReader("invalid JSON"))
			assert.Nil(t, parsed)
			assert.ErrorContains(t, e, "invalid character 'i' looking for beginning of value")
		}))
	})

	t.Run("should correctly parse the JSON content", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			scenarios := []struct {
				name     string
				content  string
				expected flam.Bag
			}{
				{
					name:     "should parse the empty objectJSON content",
					content:  `{}`,
					expected: flam.Bag{}},
				{
					name:     "should parse the simple JSON content",
					content:  `{"field": "value"}`,
					expected: flam.Bag{"field": "value"}},
				{
					name:     "should parse the multiple flat content JSON content",
					content:  `{"field": "value", "field2": "value2"}`,
					expected: flam.Bag{"field": "value", "field2": "value2"}},
				{
					name:     "should parse the multiple nested content JSON content",
					content:  `{"field": "value", "field2": {"field2": "value2"}}`,
					expected: flam.Bag{"field": "value", "field2": flam.Bag{"field2": "value2"}}},
			}

			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					parsed, e := parser.Parse(strings.NewReader(scenario.content))
					assert.Equal(t, scenario.expected, parsed)
					assert.NoError(t, e)
				})
			}
		}))
	})
}
