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

// Log logs a message at the specified level with the given message and args.
func (s Slogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	s.logger.Log(ctx, level, msg, args...)
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

func NewSlogger(logger *slog.Logger) Slogger {
	return Slogger{
		logger: logger,
	}
}
