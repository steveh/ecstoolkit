// Package log is used to initialize the logger.
package log

// T represents structs capable of logging messages.
type T interface {
	// Debug logs a message at Debug level.
	Debug(msg string, args ...any)

	// Info logs a message at Info level.
	Info(msg string, args ...any)

	// Warn logs a message at Warn level.
	Warn(msg string, args ...any)

	// Error logs a message at Error level.
	Error(msg string, args ...any)
}
