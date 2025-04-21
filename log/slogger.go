package log

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/smithy-go/logging"
)

// Slogger wraps a slog.Logger.
type Slogger struct {
	logger *slog.Logger
}

// Ensure Slogger implements the T interface.
var _ T = Slogger{}

// Log logs a message at the specified level with the given message and args.
func (s Slogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	s.logger.Log(ctx, level, msg, args...)
}

func (s Slogger) Logf(classification logging.Classification, format string, params ...any) {
	switch classification {
	case logging.Debug:
		s.Debug(fmt.Sprintf(format, params...))
	case logging.Warn:
		s.Warn(fmt.Sprintf(format, params...))
	}
}

// Tracef formats message according to format specifier
// and writes to log with level Trace.
func (s Slogger) Tracef(format string, params ...any) {
	s.Trace(fmt.Sprintf(format, params...))
}

// Debug logs a message at Debug level.
func (s Slogger) Debug(msg string, args ...any) {
	s.logger.Debug(msg, args...)
}

// Info logs a message at Info level.
func (s Slogger) Info(msg string, args ...any) {
	s.logger.Info(msg, args...)
}

// Warn logs a message at Warn level.
func (s Slogger) Warn(msg string, args ...any) {
	s.logger.Warn(msg, args...)
}

// Error logs a message at Error level.
func (s Slogger) Error(msg string, args ...any) {
	s.logger.Error(msg, args...)
}

// Infof formats message according to format specifier
// and writes to log with level Info.
func (s Slogger) Infof(format string, args ...any) {
	s.Info(fmt.Sprintf(format, args...))
}

// Warnf formats message according to format specifier
// and writes to log with level Warn.
func (s Slogger) Warnf(format string, args ...any) {
	s.Warn(fmt.Sprintf(format, args...))
}

// Errorf formats message according to format specifier
// and writes to log with level Error.
func (s Slogger) Errorf(format string, args ...any) {
	s.Error(fmt.Sprintf(format, args...))
}

// Trace writes to log with level Trace.
func (s Slogger) Trace(message string) {
	// s.logger.Debug(message)
}

func NewSlogger(logger *slog.Logger) Slogger {
	return Slogger{
		logger: logger,
	}
}
