// Package log is used to initialize the logger.
package log

import (
	"context"
	"log/slog"

	"github.com/aws/smithy-go/logging"
)

// T represents structs capable of logging messages.
type T interface {
	// Log logs a message at the specified level with the given message and args.
	Log(ctx context.Context, level slog.Level, msg string, args ...any)

	// Logf formats message according to format specifier
	// and writes to log with level Debug.
	Logf(classification logging.Classification, format string, params ...any)

	// Tracef formats message according to format specifier
	// and writes to log with level Trace.
	Tracef(format string, params ...any)

	// Debug logs a message at Debug level.
	Debug(msg string, args ...any)

	// Info logs a message at Info level.
	Info(msg string, args ...any)

	// Warn logs a message at Warn level.
	Warn(msg string, args ...any)

	// Error logs a message at Error level.
	Error(msg string, args ...any)

	// Trace writes to log with level Trace.
	Trace(message string)
}
