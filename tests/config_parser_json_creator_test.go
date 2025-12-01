package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_JsonConfigParserCreator(t *testing.T) {
	t.Run("should correctly instantiate the parser", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigParserFactory) {
			parser, e := factory.Get("my_parser")
			assert.NotNil(t, parser)
			assert.NoError(t, e)
		}))
	})
}
