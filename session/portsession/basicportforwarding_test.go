// Package portsession starts port session.
package portsession

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// This test passes ctrl+c signal which blocks running of all other tests.
//
//nolint:paralleltest // uses signal handling
func TestSetSessionHandlers(t *testing.T) {
	out, in := net.Pipe()
	defer func() {
		if err := out.Close(); err != nil {
			t.Logf("Error closing out: %v", err)
		}
	}()
	defer func() {
		if err := in.Close(); err != nil {
			t.Logf("Error closing in: %v", err)
		}
	}()

	var counter atomic.Int32

	countTimes := func() error { //nolint:unparam
		counter.Add(1)

		return nil
	}

	mockLogger := log.NewMockLog()

	mockWsChannel := getMockWsChannel()
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).
		Return(countTimes())

	mockSession := getSessionMock(t, mockWsChannel)

	portForwarding := NewBasicPortForwarding(mockSession, PortParameters{PortNumber: "22", Type: "LocalPortForwarding"}, mockLogger)
	portForwarding.acceptConnection = func(_ net.Listener) (net.Conn, error) {
		return in, nil
	}

	portSession := PortSession{
		session:         mockSession,
		portParameters:  PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: portForwarding,
		logger:          mockLogger,
	}
	signalCh := make(chan os.Signal, 1)

	go func() {
		time.Sleep(100 * time.Millisecond)

		if _, err := out.Write([]byte("testing123")); err != nil {
			mockLogger.Info("Write error", "error", err)
		}
	}()

	go func() {
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP)

		process, _ := os.FindProcess(os.Getpid())
		if err := process.Signal(syscall.SIGINT); err != nil {
			t.Logf("Error sending signal: %v", err)
		}

		if err := portSession.SetSessionHandlers(context.TODO()); err != nil {
			t.Logf("Error setting session handlers: %v", err)
		}
	}()

	time.Sleep(time.Second)
	assert.Equal(t, syscall.SIGINT, <-signalCh)
	// Verify expectations
	assert.GreaterOrEqual(t, counter.Load(), int32(1), "Expected at least one message to be sent")
	mockWsChannel.AssertExpectations(t)
}

func TestStartSessionTCPLocalPortFromDocument(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	mockWsChannel := getMockWsChannel()

	sess := getSessionMock(t, mockWsChannel)

	portForwarding := NewBasicPortForwarding(sess, PortParameters{PortNumber: "22", Type: "LocalPortForwarding"}, mockLogger)
	portForwarding.acceptConnection = func(_ net.Listener) (net.Conn, error) {
		return nil, errAcceptFailed
	}

	portSession := PortSession{
		session:         sess,
		portParameters:  PortParameters{PortNumber: "22", Type: "LocalPortForwarding", LocalPortNumber: "54321"},
		portSessionType: portForwarding,
		logger:          mockLogger,
	}
	if err := portSession.SetSessionHandlers(context.TODO()); err != nil {
		t.Logf("Error setting session handlers: %v", err)
	}

	assert.Equal(t, "54321", portSession.portParameters.LocalPortNumber)
}

func TestStartSessionTCPAcceptFailed(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockWsChannel := getMockWsChannel()

	connErr := errAcceptFailed

	sess := getSessionMock(t, mockWsChannel)

	portForwarding := NewBasicPortForwarding(sess, PortParameters{PortNumber: "22", Type: "LocalPortForwarding"}, mockLogger)
	portForwarding.acceptConnection = func(_ net.Listener) (net.Conn, error) {
		return nil, connErr
	}

	portSession := PortSession{
		session:         sess,
		portParameters:  PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: portForwarding,
		logger:          mockLogger,
	}
	require.ErrorIs(t, portSession.SetSessionHandlers(context.TODO()), connErr)
}

func TestStartSessionTCPConnectFailed(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockWsChannel := getMockWsChannel()

	listenerError := ErrConnectionFailed

	sess := getSessionMock(t, mockWsChannel)

	portForwarding := NewBasicPortForwarding(sess, PortParameters{PortNumber: "22", Type: "LocalPortForwarding"}, mockLogger)
	portForwarding.acceptConnection = func(_ net.Listener) (net.Conn, error) {
		return nil, listenerError
	}

	portSession := PortSession{
		session:         sess,
		portParameters:  PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: portForwarding,
		logger:          mockLogger,
	}
	require.ErrorIs(t, portSession.SetSessionHandlers(context.TODO()), listenerError)
}
