package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
)

func Test_StringLogSerializer(t *testing.T) {
	config := flam.Bag{}
	_ = config.Set(flam.PathLogSerializers, flam.Bag{
		"serializer": flam.Bag{
			"driver": flam.LogSerializerDriverString}})

	app := flam.NewApplication(config)
	defer func() { _ = app.Close() }()

	require.NoError(t, app.Boot())

	scenarios := []struct {
		name      string
		timestamp time.Time
		level     flam.LogLevel
		message   string
		ctx       flam.Bag
		expected  string
	}{
		{
			name:      "should serialize a log message without context",
			timestamp: time.Date(2025, time.October, 28, 11, 31, 0, 0, time.UTC),
			level:     flam.LogInfo,
			message:   "message",
			ctx:       flam.Bag{},
			expected:  "2025-10-28T11:31:00.000+0000 [INFO] message\n",
		},
		{
			name:      "should serialize a log message with context",
			timestamp: time.Date(2025, time.October, 28, 11, 31, 0, 0, time.UTC),
			level:     flam.LogInfo,
			message:   "message",
			ctx:       flam.Bag{"ctx": "value"},
			expected:  "2025-10-28T11:31:00.000+0000 [INFO] message\n",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			assert.NoError(t, app.Container().Invoke(func(factory flam.LogSerializerFactory) {
				serializer, e := factory.Get("serializer")
				require.NotNil(t, serializer)
				require.NoError(t, e)

				result := serializer.Serialize(
					scenario.timestamp,
					scenario.level,
					scenario.message,
					scenario.ctx)

				assert.Equal(t, scenario.expected, result)
			}))
		})
	}
}
