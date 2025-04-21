// Package log provides logging utilities.
package log

import (
	"context"
	"io"
	"log/slog"
)

// NoOpHandler is a slog.Handler that does nothing.
type NoOpHandler struct{}

// Handle implements slog.Handler.
func (h NoOpHandler) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

// WithAttrs implements slog.Handler.
func (h NoOpHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

// WithGroup implements slog.Handler.
func (h NoOpHandler) WithGroup(_ string) slog.Handler {
	return h
}

// Enabled implements slog.Handler.
func (h NoOpHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

// NewMockLog returns a new slog.Logger that silently discards all logs.
func NewMockLog() *slog.Logger {
	return slog.New(NoOpHandler{})
}

// This is useful for tests that need to verify log output.
func NewMockLogWithWriter(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
