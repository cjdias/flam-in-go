package log

import (
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_LogLevel(t *testing.T) {
	levels := map[flam.LogLevel]string{
		flam.LogNone: "none", flam.LogFatal: "fatal", flam.LogError: "error", flam.LogWarning: "warning",
		flam.LogNotice: "notice", flam.LogInfo: "info", flam.LogDebug: "debug",
	}

	t.Run("should name the levels", func(t *testing.T) {
		for level, name := range levels {
			testutil.Equal(t, "String()", level.String(), name)
		}
		testutil.Equal(t, "String()", flam.LogLevel(99).String(), "none")
	})

	t.Run("should not change the defaults", func(t *testing.T) {
		defaults := make([]flam.LogLevel, 1, 2)
		full := defaults[:2]
		full[1] = flam.LogError
		testutil.Equal(t, "From()", flam.LogNone.From("x", defaults...), flam.LogNone)
		testutil.Equal(t, "spare", full[1], flam.LogError)
	})

	t.Run("should resolve a level, an int or a name", func(t *testing.T) {
		for level, name := range levels {
			testutil.Equal(t, "From(level)", flam.LogNone.From(level, flam.LogInfo), level)
			testutil.Equal(t, "From(int)", flam.LogNone.From(int(level), flam.LogInfo), level)
			testutil.Equal(t, "From(name)", flam.LogNone.From(name, flam.LogInfo), level)
		}
		testutil.Equal(t, "From(\"WARNING\")", flam.LogNone.From("WARNING"), flam.LogWarning)
		for name, val := range map[string]any{
			"int8": int8(3), "int64": int64(3), "uint": uint(3), "uint8": uint8(3), "float32": float32(3),
			"float64": 3.0, "numeric string": "3", "padded numeric string": " 3 ",
		} {
			testutil.Equal(t, "From("+name+")", flam.LogNone.From(val, flam.LogInfo), flam.LogWarning)
		}
	})

	for name, value := range map[string]any{
		"level below none": flam.LogLevel(-1), "level above debug": flam.LogLevel(7),
		"int below none": -1, "int above debug": 7, "unknown name": "verbose", "fraction": 3.5,
		"float above debug": 7.0, "numeric string above debug": "7", "fractional string": "2.5", "nil": nil,
		"bool": true, "uint above debug": uint(7),
	} {
		t.Run("should fall back for a "+name, func(t *testing.T) {
			testutil.Equal(t, "From()", flam.LogNone.From(value, flam.LogInfo, flam.LogDebug), flam.LogInfo)
			testutil.Equal(t, "From() without default", flam.LogNone.From(value), flam.LogNone)
		})
	}
}
