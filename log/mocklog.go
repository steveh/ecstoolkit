package log

type MockLog struct{}

var _ T = MockLog{}

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
