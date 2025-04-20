package log

import "github.com/aws/smithy-go/logging"

type MockLog struct{}

var _ T = MockLog{}

func (m MockLog) Logf(classification logging.Classification, format string, params ...any) {
}

func (m MockLog) Tracef(format string, params ...any) {
}

func (m MockLog) Debugf(format string, params ...any) {
}

func (m MockLog) Infof(format string, params ...any) {
}

func (m MockLog) Warnf(format string, params ...any) {
}

func (m MockLog) Errorf(format string, params ...any) {
}

func (m MockLog) Trace(message string) {
}

func (m MockLog) Debug(message string) {
}

func (m MockLog) Info(message string) {
}

func (m MockLog) Warn(message string) {
}

func (m MockLog) Error(message string) {
}

func NewMockLog() *MockLog {
	return &MockLog{}
}
