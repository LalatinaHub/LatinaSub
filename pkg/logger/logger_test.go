package logger

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"dev", zerolog.DebugLevel},
		{"development", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"prod", zerolog.InfoLevel},
		{"production", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"fatal", zerolog.FatalLevel},
		{"panic", zerolog.PanicLevel},
		{"unknown", zerolog.InfoLevel},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseLogLevel(tc.input))
		})
	}
}

func TestSetupLogger(t *testing.T) {
	SetupLogger("debug", false)
	assert.NotNil(t, Logger())

	SetupLogger("info", true)
	assert.NotNil(t, Logger())

	sub := WithContext(map[string]any{"module": "test"})
	assert.NotNil(t, sub)
}
