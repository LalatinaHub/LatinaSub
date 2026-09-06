package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var globalLogger zerolog.Logger

func init() {
	SetupLogger("info", false)
}

// SetupLogger configures the global zerolog logger for LatinaSub.
func SetupLogger(levelStr string, isProduction bool) {
	level := parseLogLevel(levelStr)
	zerolog.SetGlobalLevel(level)

	var output io.Writer
	if isProduction {
		output = os.Stdout
	} else {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	globalLogger = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	log.Logger = globalLogger
}

func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug", "development", "dev":
		return zerolog.DebugLevel
	case "info", "production", "prod":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// Logger returns the global logger instance.
func Logger() *zerolog.Logger {
	return &globalLogger
}

// Info logs an info message.
func Info() *zerolog.Event {
	return globalLogger.Info()
}

// Error logs an error message.
func Error() *zerolog.Event {
	return globalLogger.Error()
}

// Debug logs a debug message.
func Debug() *zerolog.Event {
	return globalLogger.Debug()
}

// Warn logs a warning message.
func Warn() *zerolog.Event {
	return globalLogger.Warn()
}

// Fatal logs a fatal message and exits.
func Fatal() *zerolog.Event {
	return globalLogger.Fatal()
}

// WithContext returns a sub-logger enriched with given fields.
func WithContext(fields map[string]any) zerolog.Logger {
	ctx := globalLogger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return ctx.Logger()
}
