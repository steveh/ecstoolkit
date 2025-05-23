// Package portsession starts port session.
package portsession

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test StartSession.
//
//nolint:paralleltest // mutates file descriptors, not safe for parallel
func TestStartSessionForStandardStreamForwarding(t *testing.T) {
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
			session:        sess,
			portParameters: PortParameters{PortNumber: "22"},
			logger:         mockLogger,
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
