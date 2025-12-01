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

func Test_YamlConfigParser(t *testing.T) {
	t.Run("should return error on invalid reader", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})

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
			require.Nil(t, parsed)
			require.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return error on invalid YAML parse", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			parser, e := factory.Get("my_parser")
			require.NotNil(t, parser)
			require.NoError(t, e)

			parsed, e := parser.Parse(strings.NewReader("invalid YAML"))
			require.Nil(t, parsed)
			require.ErrorContains(t, e, "cannot unmarshal !!str `invalid...`")
		}))
	})

	t.Run("should correctly parse the YAML content", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})

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
					name:     "should parse the empty object YAML content",
					content:  ``,
					expected: flam.Bag{}},
				{
					name:     "should parse the simple YAML content",
					content:  `field: value`,
					expected: flam.Bag{"field": "value"}},
				{
					name:     "should parse the multiple flat content YAML content",
					content:  "field: value\nfield2: value2",
					expected: flam.Bag{"field": "value", "field2": "value2"}},
				{
					name:     "should parse the multiple nested content YAML content",
					content:  "field: value\nfield2:\n  field2: value2",
					expected: flam.Bag{"field": "value", "field2": flam.Bag{"field2": "value2"}}},
			}

			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					parsed, e := parser.Parse(strings.NewReader(scenario.content))
					require.Equal(t, scenario.expected, parsed)
					require.NoError(t, e)
				})
			}
		}))
	})
}
