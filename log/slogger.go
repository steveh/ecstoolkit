package log

import (
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

func (s Slogger) Logf(classification logging.Classification, format string, params ...any) {
	switch classification {
	case logging.Debug:
		s.Debugf(format, params...)
	case logging.Warn:
		s.Warnf(format, params...)
	}
}

// Tracef formats message according to format specifier
// and writes to log with level Trace.
func (s Slogger) Tracef(format string, params ...any) {
	s.Trace(fmt.Sprintf(format, params...))
}

// Debugf formats message according to format specifier
// and writes to log with level Debug.
func (s Slogger) Debugf(format string, params ...any) {
	s.Debug(fmt.Sprintf(format, params...))
}

// Infof formats message according to format specifier
// and writes to log with level Info.
func (s Slogger) Infof(format string, params ...any) {
	s.Info(fmt.Sprintf(format, params...))
}

// Warnf formats message according to format specifier
// and writes to log with level Warn.
func (s Slogger) Warnf(format string, params ...any) {
	s.Warn(fmt.Sprintf(format, params...))
}

// Errorf formats message according to format specifier
// and writes to log with level Error.
func (s Slogger) Errorf(format string, params ...any) {
	s.Error(fmt.Sprintf(format, params...))
}

// Trace writes to log with level Trace.
func (s Slogger) Trace(message string) {
	// s.logger.Debug(message)
}

// Debug writes to log with level Debug.
func (s Slogger) Debug(message string) {
	s.logger.Debug(message)
}

// Info writes to log with level Info.
func (s Slogger) Info(message string) {
	s.logger.Info(message)
}

// Warn writes to log with level Warn.
func (s Slogger) Warn(message string) {
	s.logger.Warn(message)
}

// Error writes to log with level Error.
func (s Slogger) Error(message string) {
	s.logger.Error(message)
}

func NewSlogger(logger *slog.Logger) Slogger {
	return Slogger{
		logger: logger,
	}
}
