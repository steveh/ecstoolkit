// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package portsession starts port session.
package portsession

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// This test passes ctrl+c signal which blocks running of all other tests.
func TestSetSessionHandlers(t *testing.T) {
	mockLog.Info("Test session started", "message", "TestStartSession!!!!!")

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

	counter := 0
	countTimes := func() error { //nolint:unparam
		counter++

		return nil
	}
	mockWebSocketChannel.On("SendMessage", mock.Anything, mock.Anything).
		Return(countTimes())

	mockSession := getSessionMock(t)
	portSession := PortSession{
		Session:        mockSession,
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: &BasicPortForwarding{
			session:        mockSession,
			portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		},
	}
	signalCh := make(chan os.Signal, 1)

	go func() {
		time.Sleep(100 * time.Millisecond)

		if _, err := out.Write([]byte("testing123")); err != nil {
			mockLog.Info("Write error", "error", err)
		}
	}()

	go func() {
		acceptConnection = func(_ net.Listener) (net.Conn, error) {
			return in, nil
		}

		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP)

		process, _ := os.FindProcess(os.Getpid())
		if err := process.Signal(syscall.SIGINT); err != nil {
			t.Logf("Error sending signal: %v", err)
		}

		if err := portSession.SetSessionHandlers(context.TODO(), mockLog); err != nil {
			t.Logf("Error setting session handlers: %v", err)
		}
	}()

	time.Sleep(time.Second)
	assert.Equal(t, syscall.SIGINT, <-signalCh)
	assert.Equal(t, 1, counter)
	mockWebSocketChannel.AssertExpectations(t)
}

func TestStartSessionTCPLocalPortFromDocument(t *testing.T) {
	acceptConnection = func(_ net.Listener) (net.Conn, error) {
		return nil, errors.New("accept failed")
	}

	portSession := PortSession{
		Session:        getSessionMock(t),
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding", LocalPortNumber: "54321"},
		portSessionType: &BasicPortForwarding{
			session:        getSessionMock(t),
			portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		},
	}
	if err := portSession.SetSessionHandlers(context.TODO(), mockLog); err != nil {
		t.Logf("Error setting session handlers: %v", err)
	}

	assert.Equal(t, "54321", portSession.portParameters.LocalPortNumber)
}

func TestStartSessionTCPAcceptFailed(t *testing.T) {
	connErr := errors.New("accept failed")
	acceptConnection = func(_ net.Listener) (net.Conn, error) {
		return nil, connErr
	}
	portSession := PortSession{
		Session:        getSessionMock(t),
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: &BasicPortForwarding{
			session:        getSessionMock(t),
			portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		},
	}
	require.ErrorIs(t, portSession.SetSessionHandlers(context.TODO(), mockLog), connErr)
}

func TestStartSessionTCPConnectFailed(t *testing.T) {
	listenerError := errors.New("TCP connection failed")
	getNewListener = func(_ string, _ string) (net.Listener, error) {
		return nil, listenerError
	}
	portSession := PortSession{
		Session:        getSessionMock(t),
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: &BasicPortForwarding{
			session:        getSessionMock(t),
			portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		},
	}
	require.ErrorIs(t, portSession.SetSessionHandlers(context.TODO(), mockLog), listenerError)
}
