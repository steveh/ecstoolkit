package log

import (
	"context"
	"log/slog"
)

// MockLog is a mock implementation of the T interface for testing purposes.
// It implements all the methods of the T interface but does not perform any actual logging.
type MockLog struct{}

var _ T = MockLog{}

// NewMockLog creates a new instance of MockLog.
func NewMockLog() *MockLog {
	return &MockLog{}
}

// Log does nothing.
func (m MockLog) Log(_ context.Context, _ slog.Level, _ string, _ ...any) {
}

// Debug does nothing.
func (m MockLog) Debug(_ string, _ ...any) {
}

// Info does nothing.
func (m MockLog) Info(_ string, _ ...any) {
}

// Warn does nothing.
func (m MockLog) Warn(_ string, _ ...any) {
}

// Error does nothing.
func (m MockLog) Error(_ string, _ ...any) {
}
