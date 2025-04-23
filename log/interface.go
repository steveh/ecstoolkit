// Package log is used to initialize the logger.
package log

import (
	"log/slog"
)

// LevelTrace represents the Trace log level.
const LevelTrace = slog.Level(-8)

// T represents structs capable of logging messages.
type T interface {
	// Trace logs a message at Trace level.
	Trace(msg string, args ...any)

	// Debug logs a message at Debug level.
	Debug(msg string, args ...any)

	// Info logs a message at Info level.
	Info(msg string, args ...any)

	// Warn logs a message at Warn level.
	Warn(msg string, args ...any)

	// Error logs a message at Error level.
	Error(msg string, args ...any)
}
