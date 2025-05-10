package log

import (
	"context"
	"log/slog"
)

// Slogger wraps a slog.Logger.
type Slogger struct {
	logger *slog.Logger
}

// Ensure Slogger implements the T interface.
var _ T = Slogger{}

// NewSlogger creates a new Slogger instance with the provided slog.Logger.
func NewSlogger(logger *slog.Logger) Slogger {
	return Slogger{
		logger: logger,
	}
}

// Trace logs a message at Trace level.
func (s Slogger) Trace(msg string, args ...any) {
	s.logger.Log(context.Background(), LevelTrace, msg, args...)
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

// With returns a new Slogger with the provided key-value pairs.
func (s Slogger) With(args ...any) T {
	return NewSlogger(s.logger.With(args...))
}
