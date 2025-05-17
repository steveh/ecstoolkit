package log

import (
	"context"
	"log/slog"
	"runtime"
	"time"
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
	ctx := context.Background()
	if !s.logger.Enabled(ctx, LevelTrace) {
		return
	}

	var pcs [1]uintptr

	runtime.Callers(2, pcs[:]) //nolint:mnd
	r := slog.NewRecord(time.Now(), LevelTrace, msg, pcs[0])
	r.Add(args...)
	_ = s.logger.Handler().Handle(ctx, r)
}

// Debug logs a message at Debug level.
func (s Slogger) Debug(msg string, args ...any) {
	ctx := context.Background()
	if !s.logger.Enabled(ctx, LevelDebug) {
		return
	}

	var pcs [1]uintptr

	runtime.Callers(2, pcs[:]) //nolint:mnd
	r := slog.NewRecord(time.Now(), LevelDebug, msg, pcs[0])
	r.Add(args...)
	_ = s.logger.Handler().Handle(ctx, r)
}

// Info logs a message at Info level.
func (s Slogger) Info(msg string, args ...any) {
	ctx := context.Background()
	if !s.logger.Enabled(ctx, LevelInfo) {
		return
	}

	var pcs [1]uintptr

	runtime.Callers(2, pcs[:]) //nolint:mnd
	r := slog.NewRecord(time.Now(), LevelInfo, msg, pcs[0])
	r.Add(args...)
	_ = s.logger.Handler().Handle(ctx, r)
}

// Warn logs a message at Warn level.
func (s Slogger) Warn(msg string, args ...any) {
	ctx := context.Background()
	if !s.logger.Enabled(ctx, LevelWarn) {
		return
	}

	var pcs [1]uintptr

	runtime.Callers(2, pcs[:]) //nolint:mnd
	r := slog.NewRecord(time.Now(), LevelWarn, msg, pcs[0])
	r.Add(args...)
	_ = s.logger.Handler().Handle(ctx, r)
}

// Error logs a message at Error level.
func (s Slogger) Error(msg string, args ...any) {
	ctx := context.Background()
	if !s.logger.Enabled(ctx, LevelError) {
		return
	}

	var pcs [1]uintptr

	runtime.Callers(2, pcs[:]) //nolint:mnd
	r := slog.NewRecord(time.Now(), LevelError, msg, pcs[0])
	r.Add(args...)
	_ = s.logger.Handler().Handle(ctx, r)
}

// With returns a new Slogger with the provided key-value pairs.
func (s Slogger) With(args ...any) T {
	return NewSlogger(s.logger.With(args...))
}
