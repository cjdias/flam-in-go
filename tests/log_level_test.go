package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cjdias/flam-in-go"
)

func Test_LogLevel(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "none", flam.LogNone.String())
		assert.Equal(t, "fatal", flam.LogFatal.String())
		assert.Equal(t, "error", flam.LogError.String())
		assert.Equal(t, "warning", flam.LogWarning.String())
		assert.Equal(t, "notice", flam.LogNotice.String())
		assert.Equal(t, "info", flam.LogInfo.String())
		assert.Equal(t, "debug", flam.LogDebug.String())
	})
}

func Test_LogLevelFrom(t *testing.T) {
	t.Run("should return the log level if passed as argument", func(t *testing.T) {
		assert.Equal(t, flam.LogNone, flam.LogLevelFrom(flam.LogNone))
		assert.Equal(t, flam.LogFatal, flam.LogLevelFrom(flam.LogFatal))
		assert.Equal(t, flam.LogError, flam.LogLevelFrom(flam.LogError))
		assert.Equal(t, flam.LogWarning, flam.LogLevelFrom(flam.LogWarning))
		assert.Equal(t, flam.LogNotice, flam.LogLevelFrom(flam.LogNotice))
		assert.Equal(t, flam.LogInfo, flam.LogLevelFrom(flam.LogInfo))
		assert.Equal(t, flam.LogDebug, flam.LogLevelFrom(flam.LogDebug))
	})

	t.Run("should return the string conversion to the proper log level", func(t *testing.T) {
		assert.Equal(t, flam.LogNone, flam.LogLevelFrom("none"))
		assert.Equal(t, flam.LogFatal, flam.LogLevelFrom("fatal"))
		assert.Equal(t, flam.LogError, flam.LogLevelFrom("error"))
		assert.Equal(t, flam.LogWarning, flam.LogLevelFrom("warning"))
		assert.Equal(t, flam.LogNotice, flam.LogLevelFrom("notice"))
		assert.Equal(t, flam.LogInfo, flam.LogLevelFrom("info"))
		assert.Equal(t, flam.LogDebug, flam.LogLevelFrom("debug"))
		assert.Equal(t, flam.LogNone, flam.LogLevelFrom("invalid"))
		assert.Equal(t, flam.LogInfo, flam.LogLevelFrom("invalid", flam.LogInfo))
	})

	t.Run("should return the int conversion to the proper log level", func(t *testing.T) {
		assert.Equal(t, flam.LogNone, flam.LogLevelFrom(0))
		assert.Equal(t, flam.LogFatal, flam.LogLevelFrom(1))
		assert.Equal(t, flam.LogError, flam.LogLevelFrom(2))
		assert.Equal(t, flam.LogWarning, flam.LogLevelFrom(3))
		assert.Equal(t, flam.LogNotice, flam.LogLevelFrom(4))
		assert.Equal(t, flam.LogInfo, flam.LogLevelFrom(5))
		assert.Equal(t, flam.LogDebug, flam.LogLevelFrom(6))
		assert.Equal(t, flam.LogNone, flam.LogLevelFrom(-1))
		assert.Equal(t, flam.LogInfo, flam.LogLevelFrom(-1, flam.LogInfo))
	})
}
