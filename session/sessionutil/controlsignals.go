// Package sessionutil contains utility methods required to start session.
package sessionutil

import (
	"os"
	"syscall"
)

// SignalsByteMap maps OS signals to their corresponding byte values.
// SIGINT captures Ctrl+C
// SIGQUIT captures Ctrl+\
// SIGTSTP captures Ctrl+Z.
//
//nolint:gochecknoglobals
var SignalsByteMap = map[os.Signal]byte{
	syscall.SIGINT:  '\003',
	syscall.SIGQUIT: '\x1c',
	syscall.SIGTSTP: '\032',
}

// ControlSignals contains the list of signals that can be used to control the session.
//
//nolint:gochecknoglobals
var ControlSignals = []os.Signal{syscall.SIGINT, syscall.SIGTSTP, syscall.SIGQUIT}
