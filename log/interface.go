// Package log is used to initialize the logger.
package log

import "github.com/aws/smithy-go/logging"

// T represents structs capable of logging messages.
type T interface {
	Logf(classification logging.Classification, format string, params ...any)

	// Tracef formats message according to format specifier
	// and writes to log with level Trace.
	Tracef(format string, params ...any)

	// Debugf formats message according to format specifier
	// and writes to log with level Debug.
	Debugf(format string, params ...any)

	// Infof formats message according to format specifier
	// and writes to log with level Info.
	Infof(format string, params ...any)

	// Warnf formats message according to format specifier
	// and writes to log with level Warn.
	Warnf(format string, params ...any)

	// Errorf formats message according to format specifier
	// and writes to log with level Error.
	Errorf(format string, params ...any)

	// Trace writes to log with level Trace.
	Trace(message string)

	// Debug writes to log with level Debug.
	Debug(message string)

	// Info writes to log with level Info.
	Info(message string)

	// Warn writes to log with level Warn.
	Warn(message string)

	// Error writes to log with level Error.
	Error(message string)
}
