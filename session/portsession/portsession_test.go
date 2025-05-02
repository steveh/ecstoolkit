// Package portsession starts port session.
package portsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/jsonutil"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	errAcceptFailed    = errors.New("accept failed")
	errHandlerNotReady = errors.New("handler not ready")
)

// Test Initialize.
func TestInitializePortSession(t *testing.T) {
	t.Parallel()

	var portParameters PortParameters

	if err := jsonutil.Remarshal(getMockProperties(), &portParameters); err != nil {
		t.Errorf("Failed to remarshal properties: %v", err)

		return
	}

	mockLogger := log.NewMockLog()

	mockWsChannel := getMockWsChannel()
	mockWsChannel.On("SetOnMessage", mock.Anything)

	sess := getSessionMock(t, mockWsChannel)
	portSession, err := NewPortSession(context.TODO(), mockLogger, sess)
	require.NoError(t, err, "Initialize port session")

	mockWsChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &StandardStreamForwarding{}, portSession.portSessionType)
}

func TestInitializePortSessionForPortForwardingWithOldAgent(t *testing.T) {
	t.Parallel()

	var portParameters PortParameters

	if err := jsonutil.Remarshal(map[string]any{"portNumber": "8080", "type": "LocalPortForwarding"}, &portParameters); err != nil {
		t.Errorf("Failed to remarshal properties: %v", err)

		return
	}

	mockLogger := log.NewMockLog()

	mockWsChannel := getMockWsChannel()
	mockWsChannel.On("SetOnMessage", mock.Anything)

	sess := getSessionMockWithParams(t, mockWsChannel, portParameters, "2.2.0.0")
	portSession, err := NewPortSession(context.TODO(), mockLogger, sess)
	require.NoError(t, err, "Initialize port session")

	mockWsChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &BasicPortForwarding{}, portSession.portSessionType)
}

func TestInitializePortSessionForPortForwarding(t *testing.T) {
	t.Parallel()

	var portParameters PortParameters

	if err := jsonutil.Remarshal(map[string]any{"portNumber": "8080", "type": "LocalPortForwarding"}, &portParameters); err != nil {
		t.Errorf("Failed to remarshal properties: %v", err)

		return
	}

	mockLogger := log.NewMockLog()

	mockWsChannel := getMockWsChannel()
	mockWsChannel.On("SetOnMessage", mock.Anything)

	sess := getSessionMockWithParams(t, mockWsChannel, portParameters, "3.1.0.0")
	portSession, err := NewPortSession(context.TODO(), mockLogger, sess)
	require.NoError(t, err, "Initialize port session")

	mockWsChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &MuxPortForwarding{}, portSession.portSessionType)
}

//nolint:paralleltest // mutates file descriptors, not safe for parallel
func TestStartSessionWithClosedWsConn(t *testing.T) {
	in, out, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Ensure cleanup
	defer func() {
		if err := in.Close(); err != nil {
			t.Logf("Error closing in: %v", err)
		}

		if err := out.Close(); err != nil {
			t.Logf("Error closing out: %v", err)
		}
	}()

	if _, err := out.Write(getMockOutputMessage().Payload); err != nil {
		t.Errorf("Failed to write to out: %v", err)

		return
	}

	os.Stdin = in

	var actualPayload []byte

	done := make(chan struct{})
	errChan := make(chan error, 1)

	// Mock SendMessage on the wsChannel
	mockWsChannel := getMockWsChannel()
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func(input []byte, _ int) error {
		actualPayload = input

		close(done)

		return nil
	})

	mockLogger := log.NewMockLog()

	sess := getSessionMock(t, mockWsChannel)
	portSession := PortSession{
		session:        sess,
		portParameters: PortParameters{PortNumber: "22"},
		portSessionType: &StandardStreamForwarding{
			inputStream:  in,
			outputStream: out,
			session:      sess,
			logger:       mockLogger,
		},
		logger: mockLogger,
	}

	// Start session handlers in a goroutine
	go func() {
		if err := portSession.SetSessionHandlers(context.TODO()); err != nil {
			errChan <- fmt.Errorf("failed to set session handlers: %w", err)

			return
		}
	}()

	// Wait for message to be processed or timeout
	select {
	case <-done:
		// Message processed successfully
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message processing")
	}

	deserializedMsg := &message.ClientMessage{}
	err = deserializedMsg.DeserializeClientMessage(actualPayload)
	require.NoError(t, err)
	assert.Equal(t, getMockOutputMessage().Payload, deserializedMsg.Payload)
}

//nolint:cyclop
func TestProcessStreamMessagePayload(t *testing.T) {
	t.Parallel()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	// Create a dummy input pipe since the method requires it
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create input pipe: %v", err)
	}

	// Create a StandardStreamForwarding instance with our test streams
	ssf := &StandardStreamForwarding{
		inputStream:  inR, // Required to pass IsStreamNotSet check
		outputStream: outW,
	}

	// Create channels for synchronization
	done := make(chan struct{})
	errChan := make(chan error, 1)
	readyChan := make(chan struct{})

	var payload []byte

	mockWsChannel := getMockWsChannel()
	mockLogger := log.NewMockLog()
	outputMessage := getMockOutputMessage()

	session := getSessionMock(t, mockWsChannel)

	go func() {
		portSession := PortSession{
			session:         session,
			portParameters:  PortParameters{PortNumber: "22"},
			portSessionType: ssf,
			logger:          mockLogger,
		}

		t.Log("Calling ProcessStreamMessagePayload")

		isReady, err := portSession.ProcessStreamMessagePayload(outputMessage)
		if err != nil {
			errChan <- fmt.Errorf("failed to process stream message payload: %w", err)

			return
		}

		if !isReady {
			errChan <- errHandlerNotReady

			return
		}

		t.Log("ProcessStreamMessagePayload succeeded")

		// Signal that we're ready to read
		close(readyChan)

		// Read the payload from the output pipe
		t.Log("Reading from output pipe")

		payload, err = io.ReadAll(outR)
		if err != nil {
			errChan <- fmt.Errorf("failed to read from output pipe: %w", err)

			return
		}

		t.Log("Successfully read from output pipe")
		close(done)
	}()

	// Close pipes when we're done
	defer func() {
		if err := inR.Close(); err != nil {
			t.Logf("Error closing inR: %v", err)
		}

		if err := inW.Close(); err != nil {
			t.Logf("Error closing inW: %v", err)
		}

		if err := outR.Close(); err != nil {
			t.Logf("Error closing outR: %v", err)
		}

		if err := outW.Close(); err != nil {
			t.Logf("Error closing outW: %v", err)
		}
	}()

	// Wait for the goroutine to be ready to read
	select {
	case <-readyChan:
		// Ready to read
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for handler to be ready")
	}

	// Close the write end of the pipe after writing
	if err := outW.Close(); err != nil {
		t.Fatalf("Failed to close output pipe: %v", err)
	}

	// Wait for processing to complete or timeout
	select {
	case <-done:
		t.Log("Test completed successfully")
		assert.Equal(t, outputMessage.Payload, payload)
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for stream processing")
	}
}
