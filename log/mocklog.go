package log

import (
	"context"
	"log/slog"

	"github.com/aws/smithy-go/logging"
)

type MockLog struct{}

var _ T = MockLog{}

func (m MockLog) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
}

func (m MockLog) Logf(classification logging.Classification, format string, params ...any) {
}

func (m MockLog) Debug(msg string, args ...any) {
}

func (m MockLog) Info(msg string, args ...any) {
}

func (m MockLog) Warn(msg string, args ...any) {
}

func (m MockLog) Error(msg string, args ...any) {
}

func NewMockLog() *MockLog {
	return &MockLog{}
}
